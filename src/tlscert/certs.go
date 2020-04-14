package tlscert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/superchalupa/sailfish/src/log"
	"golang.org/x/xerrors"
)

// Option is the type for functional options to the constructor NewCert or to reset runtime options in a cert via ApplyOption()
type Option func(*mycert) error

type mycert struct {
	certCA     *x509.Certificate
	certCApriv interface{}
	cert       *x509.Certificate
	priv       interface{}
	fileBase   string
	_logger    log.Logger
}

// NewCert constructs a new certificate object with the specified options
func NewCert(options ...Option) (*mycert, error) {
	c := &mycert{
		cert: &x509.Certificate{
			Subject: pkix.Name{
				Organization:  []string{},
				Country:       []string{},
				Province:      []string{},
				Locality:      []string{},
				StreetAddress: []string{},
				PostalCode:    []string{},
			},
			NotBefore:   time.Now(),
			NotAfter:    time.Now().AddDate(0, 0, 1), // 1 day validity unless overridden
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
			KeyUsage:    x509.KeyUsageDigitalSignature,
		},
	}

	c.ApplyOption(options...)
	return c, nil
}

func (c *mycert) logger() log.Logger {
	if c._logger == nil {
		c._logger = log.MustLogger("tlscert")
	}
	return c._logger
}

// Load will load certs from the specified file path, us SetBaseFilename() to set
func Load(options ...Option) (c *mycert, err error) {
	c = &mycert{}
	c.ApplyOption(options...)
	if c.fileBase == "" {
		panic("Key base file path not set.")
	}

	c.logger().Debug("Try to load existing Key Pair", "CRT", c.fileBase+".crt", "KEY", c.fileBase+".key")

	catls, err := tls.LoadX509KeyPair(c.fileBase+".crt", c.fileBase+".key")
	if err != nil {
		c.logger().Error("Error loading, creating new keys from scratch", "err", err)
		return
	}
	c.cert, err = x509.ParseCertificate(catls.Certificate[0])
	if err != nil {
		c.logger().Error("Error parsing certificate, creating new keys from scratch", "err", err)
		return
	}

	switch k := catls.PrivateKey.(type) {
	case *rsa.PrivateKey:
		c.priv = k
	case *ecdsa.PrivateKey:
		c.priv = k
	}

	c.logger().Info("Successfully loaded key", "filebase", c.fileBase)
	return
}

// ApplyOption will run the given option functions against the certificate.
// It's usually done implicitly in the constructor against options given on
// construction, however it can also be called at runtime to set options.
func (c *mycert) ApplyOption(options ...Option) error {
	for _, o := range options {
		err := o(c)
		if err != nil {
			return err
		}
	}
	return nil
}

// WithLogger is an option that will set up the certs function to use the specified logging interface.
func WithLogger(logger log.Logger) Option {
	return func(c *mycert) error {
		c._logger = log.With(logger, "module", "tlscert")
		return nil
	}
}

// SetBaseFilename is an option that will specify the filename to save or load certs to.
func SetBaseFilename(fn string) Option {
	return func(c *mycert) error {
		c.fileBase = fn
		return nil
	}
}

// CreateCA is an option to make this certificate a CA cert
func CreateCA(c *mycert) error {
	c.cert.IsCA = true
	c.cert.KeyUsage |= x509.KeyUsageCertSign
	c.cert.BasicConstraintsValid = true
	return nil
}

// MakeServer is an option that sets the certificate up to be used in an SSL/HTTPS server
func MakeServer(c *mycert) error {
	c.cert.KeyUsage |= x509.KeyUsageKeyEncipherment
	return nil
}

// GenRSA is an option to specify that the certificate should use an RSA key with the specified number of bits
func GenRSA(bits int) Option {
	return func(c *mycert) error {
		c.priv, _ = rsa.GenerateKey(rand.Reader, bits)
		return nil
	}
}

// GenECDSA is an option to specify that the certificate should use an RSA key with the specified number of bits
func GenECDSA(curve elliptic.Curve) Option { // elliptic.P224()
	return func(c *mycert) error {
		c.priv, _ = ecdsa.GenerateKey(curve, rand.Reader)
		return nil
	}
}

// SelfSigned is an option to specify that the generated cert will not be signed with a CA, but will be self-signed
func SelfSigned() Option {
	return func(c *mycert) error {
		// set it up as self-signed until user sets a CA
		c.certCApriv = c.priv
		c.certCA = c.cert
		return nil
	}
}

// SetSerialNumber is an option to specify that the serial number of the cert.
func SetSerialNumber(serial int64) Option {
	return func(c *mycert) error {
		c.cert.SerialNumber = big.NewInt(serial)
		return nil
	}
}

// NotBefore is an option to specify the certificates Not Valid Before attribute
func NotBefore(nb time.Time) Option {
	return func(c *mycert) error {
		c.cert.NotBefore = nb
		return nil
	}
}

// NotAfter is an option to specify the certificates Not Valid After attribute
func NotAfter(na time.Time) Option {
	return func(c *mycert) error {
		c.cert.NotAfter = na
		return nil
	}
}

// ExpireInOneYear is a helper option that specifies the expiration date as the current date + 1 year.
func ExpireInOneYear(c *mycert) error {
	c.cert.NotAfter = time.Now().AddDate(1, 0, 0)
	return nil
}

// ExpireInOneDay is a helper option that specifies that the certificate should expire in one day from today
func ExpireInOneDay(c *mycert) error {
	c.cert.NotAfter = time.Now().AddDate(1, 0, 0)
	return nil
}

// AddOrganization is an option to set the certificate Organization
func AddOrganization(org string) Option {
	return func(c *mycert) error {
		c.cert.Subject.Organization = append(c.cert.Subject.Organization, org)
		return nil
	}
}

// AddCountry is an option to set the certificate Country
func AddCountry(co string) Option {
	return func(c *mycert) error {
		c.cert.Subject.Country = append(c.cert.Subject.Country, co)
		return nil
	}
}

// AddProvince is an option to set the certificate Province
func AddProvince(prov string) Option {
	return func(c *mycert) error {
		c.cert.Subject.Province = append(c.cert.Subject.Province, prov)
		return nil
	}
}

// AddLocality is an option to set the certificate Locality
func AddLocality(loc string) Option {
	return func(c *mycert) error {
		c.cert.Subject.Locality = append(c.cert.Subject.Locality, loc)
		return nil
	}
}

// AddStreetAddress is an option to set the certificate StreetAddress
func AddStreetAddress(addr string) Option {
	return func(c *mycert) error {
		c.cert.Subject.StreetAddress = append(c.cert.Subject.StreetAddress, addr)
		return nil
	}
}

// AddPostalCode is an option to set the certificate PostalCode
func AddPostalCode(post string) Option {
	return func(c *mycert) error {
		c.cert.Subject.PostalCode = append(c.cert.Subject.PostalCode, post)
		return nil
	}
}

// SetCommonName is an option to set the certificate Common Name field
func SetCommonName(cn string) Option {
	return func(c *mycert) error {
		c.cert.Subject.CommonName = cn
		return nil
	}
}

// SetSubjectKeyID is an option to set the certificate SubjectKeyID field
func SetSubjectKeyID(id []byte) Option {
	return func(c *mycert) error {
		c.cert.SubjectKeyId = id
		return nil
	}
}

// SignWithCA will sign this certificate using the private key of the given cert
func SignWithCA(ca *mycert) Option {
	return func(c *mycert) error {
		c.certCA = ca.cert
		c.certCApriv = ca.priv
		return nil
	}
}

// AddSANDNSName will add Subject Alternate Names for the specified DNS address string
func AddSANDNSName(names ...string) Option {
	return func(c *mycert) error {
		c.cert.DNSNames = append(c.cert.DNSNames, names...)
		return nil
	}
}

// AddSANIPAddress will take the given ip addresses given as strings and parse them and add them as Subject Alternate Names for the given certificate
func AddSANIPAddress(ips ...string) Option {
	return func(c *mycert) error {
		for _, ip := range ips {
			c.cert.IPAddresses = append(c.cert.IPAddresses, net.ParseIP(ip))
		}
		return nil
	}
}

// AddSANIP will add Subject Alternate Names for the given net.IP address.
func AddSANIP(ips ...net.IP) Option {
	return func(c *mycert) error {
		c.cert.IPAddresses = append(c.cert.IPAddresses, ips...)
		return nil
	}
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

// Serialize will write the cert to files corresponding to the base filename with .crt appended (public key) and .key appended (private key).
func (c *mycert) Serialize() error {
	pub := publicKey(c.priv)
	certB, err := x509.CreateCertificate(rand.Reader, c.cert, c.certCA, pub, c.certCApriv)
	if err != nil {
		c.logger().Error("create certificate failed", "err", err)
		return errors.New("certificate creation failed")
	}

	// Public key
	certOut, err := os.OpenFile(c.fileBase+".crt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return xerrors.Errorf("certificate creation failed: failed to write public key(%s): %w", c.fileBase+".crt", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certB})
	certOut.Close()

	// Private key
	keyOut, err := os.OpenFile(c.fileBase+".key", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return xerrors.Errorf("certificate creation failed: failed to write private key(%s): %w", c.fileBase+".key", err)
	}
	pem.Encode(keyOut, pemBlockForKey(c.priv))
	keyOut.Close()
	return nil
}

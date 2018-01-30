package tlscert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

type mycert struct {
	CA     *x509.Certificate
	CApriv *rsa.PrivateKey
	cert   *x509.Certificate
	priv   *rsa.PrivateKey
}

func NewCert(options ...func(*mycert) error) (*mycert, error) {
	c := &mycert{
		cert: &x509.Certificate{
			SerialNumber: big.NewInt(1653),
			Subject: pkix.Name{
				Organization:  []string{},
				Country:       []string{},
				Province:      []string{},
				Locality:      []string{},
				StreetAddress: []string{},
				PostalCode:    []string{},
				CommonName:    "CA Cert common name",
			},
			NotBefore:   time.Now(),
			NotAfter:    time.Now().AddDate(0, 0, 1), // 1 day validity unless overridden
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
			KeyUsage:    x509.KeyUsageDigitalSignature,
		},
	}

	c.priv, _ = rsa.GenerateKey(rand.Reader, 2048)

	// set it up as self-signed until user sets a CA
	c.CApriv = c.priv
	c.CA = c.cert

	for _, o := range options {
		err := o(c)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// to make this certificate a CA cert
func CreateCA(c *mycert) error {
	c.cert.IsCA = true
	c.cert.KeyUsage = c.cert.KeyUsage | x509.KeyUsageCertSign
	c.cert.BasicConstraintsValid = true
	return nil
}

func MakeServer(c *mycert) error {
	c.cert.KeyUsage = c.cert.KeyUsage | x509.KeyUsageKeyEncipherment
	return nil
}

func NotBefore(nb time.Time) func(*mycert) error {
	return func(c *mycert) error {
		c.cert.NotBefore = nb
		return nil
	}
}

func NotAfter(na time.Time) func(*mycert) error {
	return func(c *mycert) error {
		c.cert.NotAfter = na
		return nil
	}
}

func ExpireInOneYear(c *mycert) error {
	c.cert.NotAfter = time.Now().AddDate(1, 0, 0)
	return nil
}

func ExpireInOneDay(c *mycert) error {
	c.cert.NotAfter = time.Now().AddDate(1, 0, 0)
	return nil
}

func AddOrganization(org string) func(*mycert) error {
	return func(c *mycert) error {
		c.cert.Subject.Organization = append(c.cert.Subject.Organization, org)
		return nil
	}
}

func AddCountry(co string) func(*mycert) error {
	return func(c *mycert) error {
		c.cert.Subject.Country = append(c.cert.Subject.Country, co)
		return nil
	}
}

func AddProvince(prov string) func(*mycert) error {
	return func(c *mycert) error {
		c.cert.Subject.Province = append(c.cert.Subject.Province, prov)
		return nil
	}
}

func AddLocality(loc string) func(*mycert) error {
	return func(c *mycert) error {
		c.cert.Subject.Locality = append(c.cert.Subject.Locality, loc)
		return nil
	}
}

func AddStreetAddress(addr string) func(*mycert) error {
	return func(c *mycert) error {
		c.cert.Subject.StreetAddress = append(c.cert.Subject.StreetAddress, addr)
		return nil
	}
}

func AddPostalCode(post string) func(*mycert) error {
	return func(c *mycert) error {
		c.cert.Subject.PostalCode = append(c.cert.Subject.PostalCode, post)
		return nil
	}
}

func SetCommonName(cn string) func(*mycert) error {
	return func(c *mycert) error {
		c.cert.Subject.CommonName = cn
		return nil
	}
}

func SetSubjectKeyId(id []byte) func(*mycert) error {
	return func(c *mycert) error {
		c.cert.SubjectKeyId = id
		return nil
	}
}

func Serialize(file string) func(*mycert) error {
	SAVE := func(cert *mycert) error {
		pub := &cert.priv.PublicKey
		cert_b, err := x509.CreateCertificate(rand.Reader, cert.cert, cert.CA, pub, cert.CApriv)
		if err != nil {
			log.Println("create certificate failed", err)
			return errors.New("Certificate creation failed.")
		}

		// Public key
		certOut, err := os.OpenFile(file+".crt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
		pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert_b})
		certOut.Close()

		// Private key
		keyOut, err := os.OpenFile(file+".key", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(cert.priv)})
		keyOut.Close()
		return nil
	}

	fmt.Printf("Load existing %s.crt and %s.key\n", file, file)
	catls, err := tls.LoadX509KeyPair(file+".crt", file+".key")
	if err != nil {
		fmt.Printf("\tNONEXISTENT: create from scratch\n")
		return SAVE
	}
	ca, err := x509.ParseCertificate(catls.Certificate[0])
	if err != nil {
		fmt.Printf("\tPARSE ERROR: create from scratch\n")
		return SAVE
	}

	return func(cert *mycert) error {
		fmt.Printf("\tSet to saved keys\n")
		cert.cert = ca
		cert.priv = catls.PrivateKey.(*rsa.PrivateKey)
		return nil
	}
}

func SignWithCA(ca *mycert) func(*mycert) error {
	return func(c *mycert) error {
		c.CA = ca.cert
		c.CApriv = ca.priv
		return nil
	}
}

func AddSANDNSName(names ...string) func(*mycert) error {
	return func(c *mycert) error {
		for _, name := range names {
			c.cert.DNSNames = append(c.cert.DNSNames, name)
		}
		return nil
	}
}

func AddSANIPAddress(ips ...string) func(*mycert) error {
	return func(c *mycert) error {
		for _, ip := range ips {
			c.cert.IPAddresses = append(c.cert.IPAddresses, net.ParseIP(ip))
		}
		return nil
	}
}

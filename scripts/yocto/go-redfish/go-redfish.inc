DESCRIPTION = "redfish server written in go"
SECTION = "webserver"
HOMEPAGE = "https://github.com/superchalupa/go-redfish"

DEPENDS += "go-cross-arm"

LICENSE = "apache-2.0"

# go compile doesn't set the ldflags properly to add GNU HASH. Disable this QA
# check for now until it can be addressed. Doesn't affect functionality, so
# this is safe to skip.
INSANE_SKIP_${PN} = "ldflags"

S = "${WORKDIR}/git"
UPSTREAM_GO_URI = "github.com/superchalupa/go-redfish"

SRC_URI += " \
  http://github.com/superchalupa/go-redfish/${PN}-${PV}.tar.gz	\
"

# real path should be:
#  git://github.com/superchalupa/go-redfish

do_configure[noexec] = "1"

do_compile() {
	which go
	go version

	case "${TARGET_ARCH}" in
		arm)
			GOARCH=arm
			case "${TUNE_PKGARCH}" in
				cortexa*)
					export GOARM=7
				;;
			esac
		;;
		aarch64)
			GOARCH=arm64
		;;
		i586|i686)
			GOARCH=386
		;;
		x86_64)
			GOARCH=amd64
		;;
		*)
			GOARCH="${TARGET_ARCH}"
		;;
	esac
	export GOARCH

	# Set GOPATH. See 'PACKAGERS.md'. Don't rely on
	# go to download its dependencies but rather
	# use dependencies packaged independently.
	cd ${S}
	rm -rf .gopath
	mkdir -p .gopath/src/"$(dirname "${UPSTREAM_GO_URI}")"
	ln -sf ../../../.. .gopath/src/"${UPSTREAM_GO_URI}"
	export GOPATH="${S}/.gopath"
	export GOROOT="${STAGING_LIBDIR_NATIVE}/${TARGET_SYS}/go"

	cd .gopath/src/github.com/superchalupa/go-redfish

  	$GOROOT/bin/go build -tags openbmc github.com/superchalupa/cmd/ocp-server
}

do_install() {
  install -d "${D}/${bindir}"
  install -m 0755 "${S}/redfish" "${D}/${bindir}"
}

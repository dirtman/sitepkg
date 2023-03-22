package sitepkg

import (
	"os"
	"path"
)

/*****************************************************************************\
  Provide a look and feel of a /usr/site package.  Configure the settings
  common to all /usr/site utility packages.
\*****************************************************************************/

var PkgName string
var PkgVersion string
var Package string
var PackageDir string
var PackageEtc string
var LocalEtc string
var ProgramName string
var Verbose, Quiet, Quieter, Debug bool

func PackageInit(pkg_name string, pkg_version string) error {
	PkgName = pkg_name
	PkgVersion = pkg_version
	Package = PkgName + "-" + PkgVersion
	PackageDir = "/usr/site/" + Package
	PackageEtc = PackageDir + "/etc"
	LocalEtc = "/etc/opt/" + PkgName
	ProgramName = path.Base(os.Args[0])
	SetBoolOpt("Help", "h", false, false, "Help! Show usage")
	SetBoolOpt("Verbose", "v", true, false, "Verbose mode")
	SetBoolOpt("Quiet", "q", true, false, "Quiet mode")
	SetBoolOpt("Quieter", "", true, false, "Quieter mode")
	SetBoolOpt("ShowConfig", "", false, false, "Show configuration settings and value, and exit.")
	SetBoolOpt("Page", "", true, true, "Enable paging when showing usage (-h)")
	SetStringOpt("Pager", "", true, "", "Specify a pager command for paging usage information")
	//SetStringOpt ("MailList", "m", true, "", "Specify an email address to which to email any output.")
	//SetStringOpt ("LogFile", "", true, "", "Specify a log file to which to write any output.")
	SetBoolOpt("Version", "", false, false, "Show version info.")
	return nil
}

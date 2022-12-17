package sitepkg

/*****************************************************************************\
  Functions for setting the configuration options of a program, reading in
  configuration values from configuration files, and processing command line
  options.
\*****************************************************************************/

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/pflag"
)

type Option struct {
	Type        string
	ShortOpt    string
	ConfigFile  bool
	Desc        string
	StringValue *string
	BoolValue   *bool
	IntValue    *int
	UintValue   *uint
	Source      string
}

const ConfErrNoSuchOption = "No such option"
const ConfUseConfig = "ConfUseConfig"

type Options map[string]*Option

var Config = make(Options)
var ConfigDirs []string
var PodMap = make(map[string]string)

/*****************************************************************************\
  Set up all the configuration options for the program.
  Call this function after defining all the options for the program.
  First read in options from any AND ALL config files found.
  Then parse the command line for any overrides.
\*****************************************************************************/

func ConfigureOptions() ([]string, error) {

	var args, configFiles, commandPaths []string

	ConfigDirs = []string{PackageEtc, LocalEtc, LocalEtc + "-" + PkgVersion}
	if home, err := os.UserHomeDir(); err != nil {
		Warn("Failure getting home dir: %v", err)
	} else {
		ConfigDirs = append(ConfigDirs, home+"/."+PkgName, home+"/."+Package)
	}

	if PkgName != ProgramName {
		configFiles = append(configFiles, PkgName+".conf")
	}
	if commandPaths = GetCommandPaths(); len(commandPaths) == 0 {
		return args, Error("bug: failure getting command paths")
	}
	for _, p := range commandPaths {
		configFiles = append(configFiles, p+".conf")
	}

	for _, filename := range configFiles {
		for _, pathname := range ConfigDirs {
			config_file := pathname + "/" + filename
			if _, err := os.Stat(config_file); err == nil {
				if err := ReadConfigFile(config_file); err != nil {
					return args, Error("%s!", err)
				}
			} else if !os.IsNotExist(err) {
				return args, Error("Error stat'ing config file %s: %s", config_file, err)
			}
		}
	}
	args, err := ProcessCommandLine()
	if err != nil {
		return args, err
	}

	// Set convenience globals: Verbose, Quiet, Debug.
	// Note that these options may not exist for a given program.
	debug, _ := GetBoolOpt("Debug")
	if debug {
		Debug = true
		Verbose = true
	} else {
		verbose, _ := GetBoolOpt("Verbose")
		if verbose {
			Verbose = true
		} else {
			Quiet, _ = GetBoolOpt("Quiet")
			Quieter, _ = GetBoolOpt("Quieter")
		}
	}

	// If --Help is an option, and it is set, Show Usage and exit.
	help, _ := GetBoolOpt("Help")
	if help {
		Usage()
		Exit(0)
	}

	// If --ShowConfig is an option, and it is set, ShowConfig and exit.
	show_config, _ := GetBoolOpt("ShowConfig")
	if show_config {
		ShowConfig()
		Exit(0)
	}

	// If --Version is an option, and it is set, ShowVersion and exit.
	show_version, _ := GetBoolOpt("Version")
	if show_version {
		ShowVersion()
		Exit(0)
	}
	return args, err
}

/*****************************************************************************\
  Show usage.
\*****************************************************************************/

func Usage() {
	err := ShowPod()
	if err != nil {
		Warn("Failure showing full usage: %v", err)
		Show("Usage of %s:\n", os.Args[0])
		pflag.PrintDefaults()
	}
}

/*****************************************************************************\
  Use pod2text to show the POD page for this command.
\*****************************************************************************/

func ShowPod() error {

	var pod2text, podPath, podText string
	var err error

	// First check if the caller populated PodMap.
	if podText, err = FindPodText(); err != nil {
		Warn("Failure showing POD with PodMap: %v:", err)
	} else if podText == "" {
		if podPath, err = FindPodFile(); err != nil {
			return Error("%s", err)
		} else if podPath != "" {
			if pod2text, err = ExecPath("pod2text"); err != nil {
				return Error("Failure finding pod2text command.")
			} else if pod2text == "" {
				return Error("Command pod2text not found.")
			}
		}
	}
	if podPath == "" && podText == "" {
		return Error("No POD text or POD file found")
	}

	nopage_opt, err := GetBoolOpt("noPage")
	var pager string

	if !nopage_opt {
		pager, err = GetStringOpt("Pager")
		if err != nil {
			Warn("Failure getting pager: %v", err)
		}
		if pager == "" {
			pager = os.Getenv("PAGER")
		}
	}

	if pager != "" {
		pager, err = ExecPath(pager)
	}
	if nopage_opt || pager == "" {
		if podPath == "" {
			Print("%s", podText)
			return nil
		} else {
			pod2text_command := exec.Command(pod2text, podPath)
			pod2text_command.Stdout = os.Stdout
			return pod2text_command.Run()
		}
	}
	pager_command := exec.Command(pager)
	pager_command.Stdout = os.Stdout
	pager_command.Stderr = os.Stderr

	if podPath == "" {
		pr, pw := io.Pipe()
		pager_command.Stdin = pr
		go func() {
			Fprint(pw, "%s", podText)
			pw.Close()
		}()
	} else {
		pod_command := exec.Command(pod2text, podPath)
		if pager_command.Stdin, err = pod_command.StdoutPipe(); err != nil {
			Warn("Error attaching pipe: %v", err)
		}
		go func() {
			pod_command.Run()
		}()
	}
	pager_command.Start()
	pager_command.Wait()
	return nil
}

/*****************************************************************************\
  Check if the caller populated the PodMap with an entry for the current
  command. Support subcommands, favoring, for intance, "command subcommand"
  over "command".
\*****************************************************************************/

func FindPodText() (string, error) {

	var paths []string

	// Get the list of "command" paths to search.
	if paths = GetCommandPaths(); len(paths) == 0 {
		return "", Error("bug: failure getting command paths")
	}

	// Now search the above paths in reverse order.
	for i := len(paths) - 1; i >= 0; i-- {
		path := paths[i]
		ShowDebug("FindPodText: CHECKING %s", path)
		podText, ok := PodMap[path]
		if ok && podText != "" {
			return podText, nil
		}
	}
	return "", nil
}

/*****************************************************************************\
  Search for the POD file for the current command.  Support subcommands,
  favoring, for intance, "command subcommand" over "command".
\*****************************************************************************/

func FindPodFile() (string, error) {

	var podPath, podFile string
	var paths []string
	var fileStats os.FileInfo
	var err error

	podPaths := []string{
		PackageDir + "/share/pod/pod1/",
		"/usr/share/doc/" + PkgName + "/pod1/",
		"/usr/share/doc/" + Package + "/pod1/",
	}

	// Set up the list of paths to search.
	for _, podPath = range podPaths {
		var commandPaths []string
		if commandPaths = GetCommandPaths(); len(commandPaths) == 0 {
			return "", Error("bug: failure getting command paths")
		}
		for _, command := range commandPaths {
			paths = append(paths, podPath+command)
		}
	}

	// Now search the above paths in reverse order.
	for i := len(paths) - 1; i >= 0; i-- {
		path := paths[i]
		ShowDebug("FindPod: CHECKING %s", path)
		if fileStats, err = os.Stat(path); err == nil {
			if fileStats.IsDir() {
				return "", Error("podfile \"%s\" is a directory", path)
			}
			podFile = path
			ShowDebug("FindPod: FOUND: %s", podFile)
			break
		} else if !os.IsNotExist(err) {
			return "", Error("Error stat'ing file %s: %s", path, err)
		}
	}

	if podFile == "" {
		for _, podPath = range paths {
			ShowDebug("Pod file not found: %s", podPath)
		}
		return "", Error("POD file not found.")
	}
	return podFile, nil
}

/*****************************************************************************\
  Given a command name, try /bin/path, then use PATH to search.
\*****************************************************************************/

func ExecPath(command string) (command_path string, err error) {
	command_path, err = exec.LookPath("/bin/" + command)
	if err != nil {
		command_path, err = exec.LookPath(command)
	}
	return command_path, err
}

/*****************************************************************************\

  Read in and parse the specified configuration file, and set Config options.
  Fail if an option is not recognized.  Support multiple layers of sub-commands
  via "sections", and ignore any sections that do not pertain to the invoked
  command.  For intance, for the following command:
    % ibapi host add ....
  ignore all sections except:
    ibapi  host  host:add

\*****************************************************************************/

func ReadConfigFile(config_file string) error {

	var section string
	var ignoreSection bool
	var line_no int
	var commandPaths []string

	if commandPaths = GetCommandPaths(); len(commandPaths) == 0 {
		return Error("bug: failure getting command paths")
	}
	ShowDebug("Reading config file: %s", config_file)

	file, err := os.Open(config_file)
	if err != nil {
		return Error("Error opening config file \"%s\": %v", config_file, err)
	}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line_no++
		// Trim any leading spaces:
		line := strings.TrimLeft(scanner.Text(), " \t")
		// Skip comment lines:
		if strings.HasPrefix(line, "#") {
			continue
		}
		// Skip blank lines:
		if line == "" {
			continue
		}
		// Shave off a trailing comment (must be separated from option value by at least on space):
		comment := regexp.MustCompile("[ \t]+#.*$")
		slice := comment.Split(line, 2)
		line = slice[0]

		// Check for a command section ([ptr]).
		if strings.HasPrefix(line, "[") {
			section = strings.TrimPrefix(line, "[")
			section = strings.TrimSuffix(section, "]")
			//Show("Section = %s", section)
			if section == "" {
				return Error("empty section name at line %d: %s", line_no, line)
			} else if inList, err := InList(commandPaths, section); err != nil {
				return Error("failure checking commandPath list")
			} else {
				ignoreSection = !inList
			}
			continue
		}
		if ignoreSection {
			//Show("Ignoring section = %s", section)
			continue
		}

		slice = strings.SplitN(line, "=", 2)
		if len(slice) != 2 {
			return Error("Bad line (%d) in config file %s", line_no, config_file)
		}
		option_name := strings.TrimRight(slice[0], " \t")
		option_name = strings.ToLower(option_name)
		option_value := strings.TrimLeft(slice[1], " \t")
		// Show ("option_name: \"%s\"", option_name)
		// Show ("option_value: \"%s\"", option_value)

		option, ok := Config[option_name]
		if !ok {
			return Error("Unknown option \"%s\" in config file %s", option_name, config_file)
		}
		// Show ("Current value: %s", option)
		// Show ("option_type: %s", option.Type)
		// Show ("option_file: %b", option.ConfigFile)
		if !option.ConfigFile {
			return Error("Illegal option \"%s\" in config file %s", option_name, config_file)
		}
		option.Source = "file:" + config_file
		switch option.Type {
		case "string":
			*option.StringValue = option_value
		case "int":
			*option.IntValue, err = strconv.Atoi(option_value)
			if err != nil {
				return Error("Unknown value \"%s\" specified for integer option \"%s\" in file %s",
					option_value, option_name, config_file)
			}
		case "uint":
			var var_uint uint64
			if var_uint, err = strconv.ParseUint(option_value, 10, 64); err != nil {
				return Error("Unknown value \"%s\" specified for uint option \"%s\" in file %s",
					option_value, option_name, config_file)
			}
			*option.UintValue = uint(var_uint)
		case "bool":
			option_value = strings.ToLower(option_value)
			match, _ := regexp.MatchString("^(t|true|yes|1)$", option_value)
			if match {
				*option.BoolValue = true
			} else {
				match, _ = regexp.MatchString("^(f|false|no|0)$", option_value)
				if match {
					*option.BoolValue = false
				} else {
					return Error("Unknown value \"%s\" specified for boolean option \"%s\" in file %s",
						option_value, option_name, config_file)
				}
			}
		}
	}
	if err = file.Close(); err != nil {
		return Error("Error closing config file \"%s\": %s", config_file, err)
	}
	return nil
}

/*****************************************************************************\
  Process the command line for options
\*****************************************************************************/

func ProcessCommandLine() ([]string, error) {
	var shortopt, desc string

	for name, option := range Config {
		// Show ("Config name: %s", name)
		shortopt = option.ShortOpt
		desc = option.Desc

		switch option.Type {
		case "string":
			// Show ("Config option value: %v", *option.StringValue)
			if shortopt != "" {
				pflag.StringVarP(option.StringValue, name, shortopt, *option.StringValue, desc)
			} else {
				pflag.StringVar(option.StringValue, name, *option.StringValue, desc)
			}
		case "bool":
			// Show ("Config option value: %v", *option.BoolValue)
			if shortopt != "" {
				pflag.BoolVarP(option.BoolValue, name, shortopt, *option.BoolValue, desc)
			} else {
				pflag.BoolVar(option.BoolValue, name, *option.BoolValue, desc)
			}
		case "int":
			// Show ("Config option value: %v", *option.IntValue)
			if shortopt != "" {
				pflag.IntVarP(option.IntValue, name, shortopt, *option.IntValue, desc)
			} else {
				pflag.IntVar(option.IntValue, name, *option.IntValue, desc)
			}
		case "uint":
			// Show ("Config option value: %v", *option.UintValue)
			if shortopt != "" {
				pflag.UintVarP(option.UintValue, name, shortopt, *option.UintValue, desc)
			} else {
				pflag.UintVar(option.UintValue, name, *option.UintValue, desc)
			}
		}
	}

	// Case Insensitive:
	pflag.CommandLine.SetNormalizeFunc(flagCaseInsensitive)

	// Parse the command line:
	pflag.Parse()

	// Now check which options were actually set via the command line:
	for name, option := range Config {
		if pflag.CommandLine.Changed(name) {
			option.Source = "CommandLine"
		}
	}
	return pflag.Args(), nil
}

/*****************************************************************************\
  Make command line long option flags case insensitive.  Hints taken from
  https://mymemorysucks.wordpress.com/
    2017/05/03/a-short-guide-to-mastering-strings-in-golang/
\*****************************************************************************/

func flagCaseInsensitive(f *pflag.FlagSet, name string) pflag.NormalizedName {

	// Avoid warning
	_ = f

	// Show("flagCaseInsensitive: in: \"%s\"", name)
	name_as_rune := []rune(name)
	new_name := make([]rune, 0, len(name_as_rune))

	for _, myrune := range name_as_rune {
		new_name = append(new_name, unicode.ToLower(myrune))
	}
	// Show("flagCaseInsensitive: out: %s", string(new_name))
	return pflag.NormalizedName(string(new_name))
}

/*****************************************************************************\
  Define an option of type string.
\*****************************************************************************/

func SetStringOpt(name string, shortopt string, file bool, value string, desc string) {
	var my_value string = value
	lc := strings.ToLower(name)
	Config[lc] = &Option{Type: "string", ShortOpt: shortopt, ConfigFile: file,
		Desc: desc, StringValue: &my_value, Source: "Default"}
}

/*****************************************************************************\
  Retrieve an option value of type string.
\*****************************************************************************/

func GetStringOpt(name string) (value string, err error) {
	lc := strings.ToLower(name)
	option, ok := Config[lc]
	if !ok {
		return value, Error("%s \"%s\"!", ConfErrNoSuchOption, name)
	}
	option_type := option.Type
	if option_type != "string" {
		return value, Error("GetStringOpt: bad call for %s \"%s\".", option_type, name)
	}
	return *option.StringValue, nil
}

/*****************************************************************************\
  Define an option of type bool.
\*****************************************************************************/

func SetBoolOpt(name string, shortopt string, file bool, value bool, desc string) {
	var my_value bool = value
	lc := strings.ToLower(name)
	Config[lc] = &Option{Type: "bool", ShortOpt: shortopt, ConfigFile: file,
		Desc: desc, BoolValue: &my_value, Source: "Default"}
}

/*****************************************************************************\
  Retrieve an option value of type bool.
\*****************************************************************************/

func GetBoolOpt(name string) (value bool, err error) {
	lc := strings.ToLower(name)
	option, ok := Config[lc]
	if !ok {
		return value, Error("%s \"%s\"!", ConfErrNoSuchOption, name)
	}
	option_type := option.Type
	if option_type != "bool" {
		return value, Error("GetBoolOpt: bad call for %s \"%s\".", option_type, name)
	}
	return *option.BoolValue, nil
}

/*****************************************************************************\
  Define an option of type int.
\*****************************************************************************/

func SetIntOpt(name string, shortopt string, file bool, value int, desc string) {
	var my_value int = value
	lc := strings.ToLower(name)
	Config[lc] = &Option{Type: "int", ShortOpt: shortopt, ConfigFile: file,
		Desc: desc, IntValue: &my_value, Source: "Default"}
}

/*****************************************************************************\
  Retrieve an option value of type int.
\*****************************************************************************/

func GetIntOpt(name string) (value int, err error) {
	lc := strings.ToLower(name)
	option, ok := Config[lc]
	if !ok {
		return value, Error("%s \"%s\"!", ConfErrNoSuchOption, name)
	}
	option_type := option.Type
	if option_type != "int" {
		return value, Error("GetIntOpt: bad call for %s \"%s\".", option_type, name)
	}
	return *option.IntValue, nil
}

/*****************************************************************************\
  Define an option of type uint.
\*****************************************************************************/

func SetUintOpt(name string, shortopt string, file bool, value uint, desc string) {
	var my_value uint = value
	lc := strings.ToLower(name)
	Config[lc] = &Option{Type: "uint", ShortOpt: shortopt, ConfigFile: file,
		Desc: desc, UintValue: &my_value, Source: "Default"}
}

/*****************************************************************************\
  Retrieve an option value of type uint.
\*****************************************************************************/

func GetUintOpt(name string) (value uint, err error) {
	lc := strings.ToLower(name)
	option, ok := Config[lc]
	if !ok {
		return value, Error("%s \"%s\"!", ConfErrNoSuchOption, name)
	}
	option_type := option.Type
	if option_type != "uint" {
		return value, Error("GetUintOpt: bad call for %s \"%s\".", option_type, name)
	}
	return *option.UintValue, nil
}

/*****************************************************************************\
  Print out our configuration settings and values.
\*****************************************************************************/

func ShowConfig() {
	var format, showname string
	if Debug {
		json_data, _ := json.MarshalIndent(Config, "", " ")
		Println("Configuration Details:\n%s\n", json_data)
	} else {
		format = "  %-20s "
		Println("Configurations Settings:")
		// Let's sort the options by name
		sorted_keys := make([]string, 0, len(Config))
		for name := range Config {
			sorted_keys = append(sorted_keys, name)
		}
		sort.Strings(sorted_keys)
		for _, name := range sorted_keys {
			option := Config[name]
			if option.ShortOpt == "" {
				showname = name
			} else {
				showname = name + " (-" + option.ShortOpt + ")"
			}
			switch option.Type {
			case "string":
				if len(*option.StringValue+option.Source) > 60 {
					Println(format+" \"%s\"", showname, *option.StringValue)
					Println(format+" (%s)", " ", option.Source)
				} else {
					Println(format+" \"%s\"  (%s)", showname, *option.StringValue, option.Source)
				}
			case "int":
				Println(format+" %d  (%s)", showname, *option.IntValue, option.Source)
			case "uint":
				Println(format+" %d  (%s)", showname, *option.UintValue, option.Source)
			case "bool":
				Println(format+" %v  (%s)", showname, *option.BoolValue, option.Source)
			}
		}
	}
}

func ShowVersion() {
	Println("Version info for %s:", ProgramName)
	Println("  PkgName: %s", PkgName)
	Println("  PkgVersion: %s", PkgVersion)
	Println("  PackageEtc: %s", PackageEtc)
	Println("  LocalEtc: %s", LocalEtc)
}

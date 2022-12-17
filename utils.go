package sitepkg

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

/*****************************************************************************\
  Exit the program.  Improve upon later.
\*****************************************************************************/

func Exit(code int) {
	os.Exit(code)
}

/*****************************************************************************\
  Convenience wrapper for errors.New().
\*****************************************************************************/

func Error(format string, a ...interface{}) error {
	return errors.New(fmt.Sprintf(format, a...))
}

/*****************************************************************************\
  Check if the specified file exists.  Improve upon later.
\*****************************************************************************/

func FileExists(filename string) (exists bool, err error) {
	if filename == "" {
		return false, Error("Bad call: filename not defined.")
	}
	if _, err = os.Stat(filename); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, Error("Error stat'ing file %s: %s", filename, err)
}

/*****************************************************************************\
  Find a file by searching in the "standard package places", from highest
  priority to lowest.  Return only the first one found.
\*****************************************************************************/

func FindPackageFile(filename string) (pathname string, err error) {

	if filename == "" {
		return "", Error("Bad call: filename not defined.")
	}
	if strings.HasPrefix(filename, "/") || strings.HasPrefix(filename, "./") {
		if exists, err := FileExists(filename); err != nil {
			return "", err
		} else if !exists {
			return "", Error("No such file \"%s\".", filename)
		}
		return filename, nil
	}

	for i := len(ConfigDirs) - 1; i >= 0; i-- {
		pathname := ConfigDirs[i] + "/" + filename
		if exists, err := FileExists(pathname); err != nil {
			return "", err
		} else if exists {
			return pathname, nil
		}
	}
	return "", Error("File \"%s\" not found", filename)
}

/*****************************************************************************\
  Read a list of strings from a file searched for in the standard places.
\*****************************************************************************/

func ReadListFromPkgFile(filename string) (list []string, err error) {
	if filename == "" {
		return list, Error("Bad call: filename not defined.")
	}
	pathname, err := FindPackageFile(filename)
	if err != nil {
		return nil, err
	}
	return ReadListFromFile(pathname)
}

/*****************************************************************************\
  Read a list of strings from a file.
\*****************************************************************************/

func ReadListFromFile(filename string) (list []string, err error) {

	if filename == "" {
		return list, Error("Bad call: filename not defined.")
	} else if exists, err := FileExists(filename); err != nil {
		return nil, err
	} else if !exists {
		return nil, Error("No such file \"%s\".", filename)
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, Error("Error opening file \"%s\": %v", filename, err)
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Remove leading spaces and tabs:
		line := strings.TrimLeft(scanner.Text(), " \t")
		// Skip comment (#) lines:
		if strings.HasPrefix(line, "#") {
			continue
		} else if line == "" {
			continue
		}
		// Define a trailing comment: one or more spaces/tabs followed by '#.*':
		comment := regexp.MustCompile("[ \t]+#.*$")
		// Split line into a slice of at most 2 strings, spitting by our regexp:
		slice := comment.Split(line, 2)
		// Append our trimmed line to the list:
		list = append(list, strings.TrimRight(slice[0], " \t"))
	}
	return list, nil
}

/*****************************************************************************\
  Read a "secret", presumably a password, from a "secret file".  The idea is to
  make stored passwords a little safer by storing them in protected files, as
  opposed to a general configuration file.  Returns the first secret found.
\*****************************************************************************/

func GetSecret(account string) (string, error) {

	if account == "" {
		return "", Error("Bad call: account not defined.")
	}
	var secrets_dir, filename string

	if secrets_dir, _ = GetStringOpt("SecretsDir"); secrets_dir == "" {
		if filename, _ = FindPackageFile("private/" + account); filename == "" {
			return "", Error("Credentials file \"%s\" not found.", account)
		}
	} else {
		filename = secrets_dir + "/" + account
	}

	list, err := ReadListFromFile(filename)
	if err != nil {
		return "", err
	} else if list == nil {
		return "", Error("Failure reading secret from secrets file \"%s\".", filename)
	}
	return list[0], nil
}

/*****************************************************************************\
  Check if the specified string is the the list of strings.
\*****************************************************************************/

func InList(list []string, check_item string) (in_list bool, err error) {
	if list == nil {
		return false, nil
	}
	for _, item := range list {
		if item == check_item {
			return true, nil
		}
	}
	return false, nil
}

/*****************************************************************************\

  Return true if user_value is not empty and is equivalent to resource_value.
  Return false if user_value is not empty and in not equivalent.
  If user_value is empty, return the value of "not_specified".

  "user_value" may have a special prefix of "not:"; if so, strip that prefix
  from user_value and return true if the stripped value does NOT equal
  resource_value.

  Parameters:
  user_value: an option value presumably specified by the user (i.e.: -s deleted).
  resource_value: the value of the corresponding resource attribute.
  not_specified: result value if the user does not specify a value for the option.

\*****************************************************************************/

func CheckFlagValue(user_value string, resource_value string, not_specified bool) bool {

	// Show ("user_value: \"%s\".", user_value)
	// Show ("resource_value: \"%s\".", resource_value)

	if user_value == "" {
		return not_specified
	} else if strings.HasPrefix(user_value, "not:") {
		user_value = strings.TrimLeft(user_value, "not:")
		return !strings.EqualFold(user_value, resource_value)
	}
	return strings.EqualFold(user_value, resource_value)
}

/*****************************************************************************\
  GetCommandPaths returns the list of "paths" for the invoked
  command/sub-commands. For instance, if the user invoked "ibapi host
  add host.com 10.10.10.10", and "add" is the final sub-command, the
  following list is returned: ["ibapi", "host", "host:add" ].
\*****************************************************************************/

func GetCommandPaths() []string {

	var paths []string
	var sep, command string

	// Set up the list of paths to search.
	paths = append(paths, ProgramName)
	for _, c := range strings.Split(os.Args[0], " ")[1:] {
		command += sep + c
		sep = ":"
		paths = append(paths, command)
	}
	return paths
}

/*****************************************************************************\
  Convenience func for converting a string to a uint.
\*****************************************************************************/

func StringToUint(s string, b int) (uint, error) {

	if b == 0 {
		b = 32
	}

	if u64, err := strconv.ParseUint(s, 10, b); err != nil {
		return 0, err
	} else {
		return uint(u64), nil
	}
}

/*****************************************************************************\
  Convenience func for converting a string to a bool.
\*****************************************************************************/

func StringToBool(s string) (match bool, err error) {

	if match, err = regexp.MatchString("^(t|true|yes|1)$", s); match || err != nil {
		return match, err
	} else if match, err = regexp.MatchString("^(f|false|no|0)$", s); err != nil {
		return match, err
	} else if !match {
		return false, Error("unsupported string \"%s\" for boolean value")
	}
	return ! match, nil
}



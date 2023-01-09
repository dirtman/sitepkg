package sitepkg

/*****************************************************************************\
  Functions for outputing warning or informational messages.  We want all
  output to go through us, so we can divert to mail, syslog, etc as desired.
\*****************************************************************************/

import (
	"fmt"
	"io"
	"log"
	"os"
)

var DefaultPrint io.Writer = os.Stdout
var DefaultShow io.Writer = os.Stdout
var DefaultErr io.Writer = os.Stderr

func Print(format string, a ...interface{}) {
	fmt.Fprintf(DefaultPrint, format, a...)
}

func Println(format string, a ...interface{}) {
	fmt.Fprintf(DefaultPrint, format+"\n", a...)
}

func Show(format string, a ...interface{}) {
	myformat := ProgramName + ": " + format
	fmt.Fprintf(DefaultShow, myformat+"\n", a...)
}

func Warn(format string, a ...interface{}) {
	myformat := ProgramName + ": Warning: " + format
	fmt.Fprintf(DefaultErr, myformat+"\n", a...)
}

func Fprint(w io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(w, format, a...)
}

func Fprintln(w io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(w, format+"\n", a...)
}

func Fshow(w io.Writer, format string, a ...interface{}) {
	myformat := ProgramName + ": " + format
	Fprintln(w, myformat, a...)
}

func Fwarn(w io.Writer, format string, a ...interface{}) {
	myformat := ProgramName + ": Warning: " + format
	fmt.Fprintf(w, myformat+"\n", a...)
}

func ShowDebug(format string, a ...interface{}) {
	if Debug {
		fmt.Fprintf(DefaultShow, "DEBUG: "+format+"\n", a...)
	}
}

func Log(format string, a ...interface{}) {
	log.Printf(format, a...)
}


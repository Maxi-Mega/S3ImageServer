package main

import (
	"fmt"
	"os"
	"time"
)

func log(a ...interface{}) {
	if config.Debug {
		fmt.Print(time.Now().Format("2006-01-02 15:04:05"), " ")
		fmt.Println(a...)
	}
}

func printError(err error, fatal bool) {
	if fatal {
		fmt.Println("\n/!\\ Fatal Error /!\\")
	} else {
		fmt.Println("\n/!\\    Error    /!\\")
	}
	fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("- - - - - - - - - -")
	fmt.Println(err)
}

func exitWithError(err error) {
	printError(err, true)
	os.Exit(1)
}

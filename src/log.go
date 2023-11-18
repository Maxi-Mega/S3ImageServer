package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	levelDebug = "debug"
	levelInfo  = "info"
	levelWarn  = "warn"
	levelError = "error"
)

const logTimeFormat = "2006-01-02T15:04:05.000Z"

var logger zerolog.Logger

func init() {
	zerolog.TimestampFieldName = "date"
	zerolog.TimeFieldFormat = logTimeFormat
	for _, v := range []*string{&zerolog.LevelTraceValue, &zerolog.LevelDebugValue, &zerolog.LevelInfoValue, &zerolog.LevelWarnValue, &zerolog.LevelErrorValue, &zerolog.LevelFatalValue, &zerolog.LevelPanicValue} {
		*v = strings.ToUpper(*v)
	}
	// This logger will only be used until initLogger() is called.
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		NoColor:    true,
		TimeFormat: logTimeFormat,
	}
	logger = zerolog.New(consoleWriter).With().Timestamp().Logger()
	// {"date":"2021-12-16T15:07:48.264Z","level":"INFO","class_name":"activate_production","service_name":"s950","message":"Creating Zeebe client"}
}

func initLogger() {
	if config.JsonLogFormat {
		ctx := zerolog.New(os.Stdout).With().Timestamp()
		for key, value := range config.JsonLogFields {
			ctx = ctx.Interface(key, value)
		}
		logger = ctx.Logger()
	} else {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			NoColor:    !config.ColorLogs,
			TimeFormat: logTimeFormat,
		}
		logger = zerolog.New(consoleWriter).With().Timestamp().Logger()
	}
}

func printDebug(a ...interface{}) {
	if config.LogLevel == levelDebug {
		logger.Debug().Msg(fmt.Sprint(a...))
	}
}

func printInfo(a ...interface{}) {
	if config.LogLevel == levelDebug || config.LogLevel == levelInfo {
		logger.Info().Msg(fmt.Sprint(a...))
	}
}

func printWarn(a ...interface{}) {
	if config.LogLevel == levelDebug || config.LogLevel == levelInfo || config.LogLevel == levelWarn {
		logger.Warn().Msg(fmt.Sprint(a...))
	}
}

func printError(err error, fatal bool) {
	if config.JsonLogFormat {
		if fatal {
			logger.Fatal().Msg(err.Error())
		} else {
			logger.Error().Msg(err.Error())
		}
	} else {
		if fatal {
			fmt.Println("\n/!\\ Fatal Error /!\\")
		} else {
			fmt.Println("\n/!\\    Error    /!\\")
		}
		fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
		fmt.Println("- - - - - - - - - -")
		fmt.Println(err)
	}

	/*bytes, err := json.Marshal(logObj)
	if err != nil {
		fmt.Println(logObj)
		os.Exit(1)
	}
	fmt.Println(string(bytes))*/
}

func exitWithError(err error) {
	printError(err, true)
	os.Exit(1)
}

func handleS3Error(err error) {
	if config.ExitOnS3Error {
		exitWithError(err)
	}
}

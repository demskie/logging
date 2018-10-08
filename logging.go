package logging

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
)

// ansi colors
const (
	RED int8 = iota
	CYAN
	WHITE
	MAGENTA
)

const oneMillion int = 1000000

type loggingMessage struct {
	color   int8
	theTime time.Time
	message string
}

// Logger has methods to create new log messages
type Logger struct {
	debugMode bool
	logChan   chan loggingMessage
}

var globalLogger *Logger
var globalMutex sync.Mutex

// New only creates a new logger if one does not already exist
func New() *Logger {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	if globalLogger != nil {
		return globalLogger
	}
	globalLogger = &Logger{
		debugMode: false,
		logChan:   make(chan loggingMessage, oneMillion),
	}
	globalLogger.startLogging()
	go func() {
		closing := make(chan os.Signal, 1)
		signal.Notify(closing, os.Interrupt)
		<-closing
		globalMutex.Lock()
		fmt.Print("\033[0m\n\r")
		os.Exit(1)
	}()
	return globalLogger
}

// EnableDebugMode is used to show Debug, Dump and Telemetry messages
func (l *Logger) EnableDebugMode() {
	l.debugMode = true
}

const defaultTimeLayout string = "2006-01-02 15:04:05"

func (l *Logger) startLogging() {
	var (
		i                      int
		aColor                 int8
		notClosed, exists      bool
		aMsg                   string
		earliestTime, thisTime time.Time
		multipleTimes          []time.Time
		firstLog, nextLog      loggingMessage
		newLogs                map[int8]map[string][]time.Time
	)
	go func() {
		for {
			firstLog = <-l.logChan
			newLogs = map[int8]map[string][]time.Time{}
			newLogs[firstLog.color] = map[string][]time.Time{}
			newLogs[firstLog.color][firstLog.message] = []time.Time{firstLog.theTime}
			for i = 0; i < 100000; i++ {
				select {
				case nextLog, notClosed = <-l.logChan:
					if notClosed {
						_, exists = newLogs[nextLog.color]
						if exists == false {
							newLogs[nextLog.color] = map[string][]time.Time{}
						}
						_, exists = newLogs[nextLog.color][nextLog.message]
						if exists == false {
							newLogs[nextLog.color][nextLog.message] = []time.Time{}
						}
						newLogs[nextLog.color][nextLog.message] = append(
							newLogs[nextLog.color][nextLog.message], nextLog.theTime)
					} else {
						panic("*Logger.logChan has closed unexpectedly")
					}
				default:
					break
				}
			}
			globalMutex.Lock()
			for aColor = range newLogs {
				for aMsg, multipleTimes = range newLogs[aColor] {
					if len(multipleTimes) > 1 {
						earliestTime = multipleTimes[0]
						for _, thisTime = range multipleTimes {
							if thisTime.Before(earliestTime) {
								earliestTime = thisTime
							}
						}
						aMsg = (earliestTime.UTC().Format(defaultTimeLayout) + " " +
							aMsg + " (repeated " + fmt.Sprint(len(multipleTimes)) + " times)")
					} else {
						aMsg = multipleTimes[0].UTC().Format(defaultTimeLayout) + " " + aMsg
					}
					switch aColor {
					case RED:
						fmt.Printf("\033[1;31m%s\033[0m\n\r", aMsg)
					case CYAN:
						fmt.Printf("\033[1;36m%s\033[0m\n\r", aMsg)
					case WHITE:
						fmt.Printf("\033[1;37m%s\033[0m\n\r", aMsg)
					case MAGENTA:
						if l.debugMode {
							fmt.Printf("\033[1;35m%s\033[0\n\r", aMsg)
						}
					}
				}
			}
			globalMutex.Unlock()
			time.Sleep(time.Second)
		}
	}()
}

func concatenateMessages(words []string) (msg string) {
	for i, val := range words {
		if i > 0 {
			msg += " "
		}
		msg += val
	}
	return
}

// CheckError logs error if it is not nil and returns a boolean of the result
func (l *Logger) CheckError(err error, words ...string) bool {
	if err == nil {
		return false
	}
	msg := concatenateMessages(words)
	l.logChan <- loggingMessage{
		color:   RED,
		theTime: time.Now(),
		message: "/e " + msg + " (err) " + err.Error(),
	}
	return true
}

// Error logs an error with the strings specified
func (l *Logger) Error(words ...string) {
	msg := concatenateMessages(words)
	l.logChan <- loggingMessage{
		color:   RED,
		theTime: time.Now(),
		message: "/e " + msg,
	}
}

// Debug logs a debug message with the strings specified
func (l *Logger) Debug(words ...string) {
	if l.debugMode == false {
		return
	}
	msg := time.Now().UTC().Format(defaultTimeLayout)
	msg += " /d " + concatenateMessages(words)
	globalMutex.Lock()
	fmt.Printf("\033[1;32m%s\033[0m\n\r", msg)
	globalMutex.Unlock()
}

// Dump logs a debug using the davecgh/go-spew package for formatting
func (l *Logger) Dump(iface ...interface{}) {
	l.Debug(spew.Sdump(iface))
}

// Info logs an informational message with the strings specified
func (l *Logger) Info(words ...string) {
	msg := concatenateMessages(words)
	l.logChan <- loggingMessage{
		color:   WHITE,
		theTime: time.Now(),
		message: "/i " + msg,
	}
}

// Telemetry logs a telemetry message with the strings specified
func (l *Logger) Telemetry(words ...string) {
	msg := concatenateMessages(words)
	l.logChan <- loggingMessage{
		color:   MAGENTA,
		theTime: time.Now(),
		message: "/t " + msg,
	}
}

// SuddenExit starts the shutdown process
func (l *Logger) SuddenExit() {
	i := 0
	time.Sleep(time.Second)
	for len(l.logChan) != 0 {
		l.Error("(suddenExit) shutting down in", fmt.Sprint(15-i), "seconds")
		time.Sleep(time.Second)
		if i >= 15 {
			break
		}
		i++
	}
	t := time.Now().UTC().Format(defaultTimeLayout)
	globalMutex.Lock()
	fmt.Printf("\033[1;31m%s\033[0m /e (suddenExit) program has finished executing\n\r", t)
	os.Exit(1)
}

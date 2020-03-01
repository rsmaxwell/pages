package debug

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rsmaxwell/page/internal/config"
	"github.com/rsmaxwell/page/internal/version"
)

// Package type
type Package struct {
	name  string
	level int
}

// Function type
type Function struct {
	pkg   *Package
	name  string
	level int
}

const (
	// ErrorLevel trace level
	ErrorLevel = 10

	// WarningLevel trace level
	WarningLevel = 20

	// InfoLevel trace level
	InfoLevel = 30

	// APILevel trace level
	APILevel = 40

	// VerboseLevel trace level
	VerboseLevel = 50

	minUint uint = 0 // binary: all zeroes

	maxUint = ^minUint // binary: all ones

	maxInt = int(maxUint >> 1) // binary: all ones except high bit

	minInt = ^maxInt // binary: all zeroes except high bit

)

var (
	file                 *os.File
	logger               *log.Logger
	level                int
	defaultPackageLevel  int
	defaultFunctionLevel int
	dumpRoot             string
	functionLevels       map[string]int
	packageLevels        map[string]int
)

// Open function
func Open(c config.Debug) {
	level = c.Level
	defaultPackageLevel = c.DefaultPackageLevel
	defaultFunctionLevel = c.DefaultFunctionLevel
	dumpRoot = c.DumpDir

	functionLevels = c.FunctionLevels
	packageLevels = c.PackageLevels

	os.MkdirAll(dumpRoot, 0755)

	var err error
	file, err = os.OpenFile("text.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	logger = log.New(file, "page", log.LstdFlags)
}

// Close function
func Close() {
	file.Close()
}

// NewPackage function
func NewPackage(name string) *Package {
	m := &Package{name: name, level: defaultPackageLevel}

	value, ok := packageLevels[name]
	if ok {
		m.level = value
	}

	return m
}

// NewFunction function
func NewFunction(pkg *Package, name string) *Function {
	d := &Function{pkg: pkg, name: name, level: defaultFunctionLevel}

	value, ok := functionLevels[pkg.name+"_"+name]
	if ok {
		d.level = value
	}

	return d
}

// --------------------------------------------------------

// DebugError prints an 'error' message
func (f *Function) DebugError(format string, a ...interface{}) {
	f.Debug(ErrorLevel, format, a...)
}

// DebugWarn prints an 'warning' message
func (f *Function) DebugWarn(format string, a ...interface{}) {
	f.Debug(WarningLevel, format, a...)
}

// DebugInfo prints an 'info' message
func (f *Function) DebugInfo(format string, a ...interface{}) {
	f.Debug(InfoLevel, format, a...)
}

// DebugAPI prints an 'error' message
func (f *Function) DebugAPI(format string, a ...interface{}) {
	f.Debug(APILevel, format, a...)
}

// DebugVerbose prints an 'error' message
func (f *Function) DebugVerbose(format string, a ...interface{}) {
	f.Debug(VerboseLevel, format, a...)
}

// --------------------------------------------------------

// Errorf prints an 'error' message
func (f *Function) Errorf(format string, a ...interface{}) {
	f.Println(ErrorLevel, format, a...)
}

// Warnf prints an 'warning' message
func (f *Function) Warnf(format string, a ...interface{}) {
	f.Println(WarningLevel, format, a...)
}

// Infof prints an 'info' message
func (f *Function) Infof(format string, a ...interface{}) {
	f.Println(InfoLevel, format, a...)
}

// APIf prints an 'error' message
func (f *Function) APIf(format string, a ...interface{}) {
	f.Println(APILevel, format, a...)
}

// Verbosef prints an 'error' message
func (f *Function) Verbosef(format string, a ...interface{}) {
	f.Println(VerboseLevel, format, a...)
}

// --------------------------------------------------------

// Fatalf prints a 'fatal' message
func (f *Function) Fatalf(format string, a ...interface{}) {
	f.Debug(ErrorLevel, format, a...)
	os.Exit(1)
}

// Debug prints the function name
func (f *Function) Debug(l int, format string, a ...interface{}) {
	if l <= level {
		if l <= f.pkg.level {
			if l <= f.level {
				line1 := fmt.Sprintf(format, a...)
				line2 := fmt.Sprintf("%s.%s %s", f.pkg.name, f.name, line1)
				logger.Printf(line2)
			}
		}
	}
}

// Printf prints a debug message
func (f *Function) Printf(l int, format string, a ...interface{}) {
	if l <= level {
		if l <= f.pkg.level {
			if l <= f.level {
				logger.Printf(format, a...)
			}
		}
	}
}

// Println prints a debug message
func (f *Function) Println(l int, format string, a ...interface{}) {
	if l <= level {
		if l <= f.pkg.level {
			if l <= f.level {
				logger.Println(fmt.Sprintf(format, a...))
			}
		}
	}
}

// Level returns the effective trace level
func (f *Function) Level() int {

	effectiveLevel := maxInt

	if level < effectiveLevel {
		effectiveLevel = level
	}

	if f.pkg.level < effectiveLevel {
		effectiveLevel = f.pkg.level
	}

	if f.level < effectiveLevel {
		effectiveLevel = f.level
	}

	return effectiveLevel
}

// DebugRequest traces the http request
func (f *Function) DebugRequest(req *http.Request) {

	if f.Level() >= APILevel {
		f.DebugAPI("%s %s %s %s", req.Method, req.Proto, req.Host, req.URL)

		for name, headers := range req.Header {
			name = strings.ToLower(name)
			for _, h := range headers {
				f.DebugAPI("%v: %v", name, h)
			}
		}
	}
}

// DebugRequestBody traces the http request body
func (f *Function) DebugRequestBody(data []byte) {

	if f.Level() >= APILevel {
		text1 := string(data) // multi-line json

		space := regexp.MustCompile(`\s+`)
		text2 := space.ReplaceAllString(text1, " ") // may contain a 'password' field

		text3 := text2
		var m map[string]interface{}
		err := json.Unmarshal([]byte(text2), &m)
		if err == nil {
			text3 = "{ "
			sep := ""
			for k, v := range m {
				v2 := v
				if strings.ToLower(k) == "password" {
					v2 = interface{}("********")
				}
				text3 = fmt.Sprintf("%s%s\"%s\": \"%s\"", text3, sep, k, v2)
				sep = ", "
			}
			text3 = text3 + " }"
		}
		f.DebugAPI("request body: %s", text3) // sanitised!
	}
}

// Dump type
type Dump struct {
	directory string
	err       error
}

// DumpInfo type
type DumpInfo struct {
	GroupID       string `json:"groupidid"`
	Artifact      string `json:"artifact"`
	RepositoryURL string `json:"repositoryurl"`
	Timestamp     string `json:"timestamp"`
	TimeUnix      int64  `json:"timeunix"`
	TimeUnixNano  int64  `json:"timeunixnano"`
	Package       string `json:"package"`
	Function      string `json:"function"`
	FuncForPC     string `json:"funcforpc"`
	Filename      string `json:"filename"`
	Line          int    `json:"line"`
	Version       string `json:"version"`
	BuildDate     string `json:"builddate"`
	GitCommit     string `json:"gitcommit"`
	GitBranch     string `json:"gitbranch"`
	GitURL        string `json:"giturl"`
	Message       string `json:"message"`
}

// Dump function
func (f *Function) Dump(format string, a ...interface{}) *Dump {

	dump := new(Dump)

	t := time.Now()
	now := fmt.Sprintf(t.Format("2006-01-02_15-04-05.999999999"))
	dump.directory = dumpRoot + "/" + now

	f.DebugError("DUMP: writing dump:[%s]", dump.directory)
	err := os.MkdirAll(dump.directory, 0755)
	if err != nil {
		dump.err = err
		return dump
	}

	pc, fn, line, ok := runtime.Caller(1)
	if ok {
		fmt.Println(fmt.Sprintf("package.function: %s.%s", f.pkg.name, f.name))
		fmt.Println(fmt.Sprintf("package.function: %s", runtime.FuncForPC(pc).Name()))
		fmt.Println(fmt.Sprintf("filename: %s[%d]", fn, line))
	}

	// *****************************************************************
	// * Main dump info
	// *****************************************************************
	info := new(DumpInfo)
	info.GroupID = "com.rsmaxwell.players"
	info.Artifact = "players-api"
	info.RepositoryURL = "https://server.rsmaxwell.co.uk/archiva"
	info.Timestamp = now
	info.TimeUnix = t.Unix()
	info.TimeUnixNano = t.UnixNano()
	info.Package = f.pkg.name
	info.Function = f.name
	info.FuncForPC = runtime.FuncForPC(pc).Name()
	info.Filename = fn
	info.Line = line
	info.Version = version.Version()
	info.BuildDate = version.BuildDate()
	info.GitCommit = version.GitCommit()
	info.GitBranch = version.GitBranch()
	info.GitURL = version.GitURL()
	info.Message = fmt.Sprintf(format, a...)

	json, err := json.Marshal(info)
	if err != nil {
		dump.err = err
		return dump
	}

	filename := dump.directory + "/dump.json"

	err = ioutil.WriteFile(filename, json, 0644)
	if err != nil {
		dump.err = err
		return dump
	}

	// *****************************************************************
	// * Call stack
	// *****************************************************************
	stacktrace := debug.Stack()
	filename = dump.directory + "/callstack.txt"

	err = ioutil.WriteFile(filename, stacktrace, 0644)
	if err != nil {
		dump.err = err
		return dump
	}

	return dump
}

// AddByteArray method
func (dump *Dump) AddByteArray(title string, data []byte) *Dump {

	if dump.err != nil {
		return dump
	}

	filename := dump.directory + "/" + title

	err := ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		dump.err = err
		return dump
	}

	return dump
}

// MarkDumps type
type MarkDumps struct {
	dumps map[string]bool
	err   error
}

// Mark method
func Mark() *MarkDumps {

	mark := new(MarkDumps)

	files, err := ioutil.ReadDir(dumpRoot)
	if err != nil {
		mark.err = err
		return mark
	}

	mark.dumps = map[string]bool{}

	for _, file := range files {
		if file.IsDir() {
			mark.dumps[file.Name()] = true
		}
	}

	return mark
}

// ListNewDumps method
func (mark *MarkDumps) ListNewDumps() ([]*Dump, error) {

	if mark.err != nil {
		return nil, mark.err
	}

	files, err := ioutil.ReadDir(dumpRoot)
	if err != nil {
		mark.err = err
		return nil, err
	}

	newdumps := []*Dump{}

	for _, file := range files {
		if file.IsDir() {
			if !mark.dumps[file.Name()] {

				dump := new(Dump)
				dump.directory = dumpRoot + "/" + file.Name()

				newdumps = append(newdumps, dump)
			}
		}
	}

	return newdumps, nil
}

// ListDumps method
func ListDumps() ([]*Dump, error) {

	files, err := ioutil.ReadDir(dumpRoot)
	if err != nil {
		return nil, err
	}

	newdumps := []*Dump{}

	for _, file := range files {
		if file.IsDir() {
			dump := new(Dump)
			dump.directory = dumpRoot + "/" + file.Name()

			newdumps = append(newdumps, dump)
		}
	}

	return newdumps, nil
}

// Remove function
func (dump *Dump) Remove() error {

	err := os.RemoveAll(dump.directory)
	if err != nil {
		return err
	}

	return nil
}

// GetInfo function
func (dump *Dump) GetInfo() (*DumpInfo, error) {

	infofile := dump.directory + "/dump.json"

	data, err := ioutil.ReadFile(infofile)
	if err != nil {
		return nil, err
	}

	var info DumpInfo
	err = json.Unmarshal(data, &info)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

// ClearDumps function
func ClearDumps() error {

	dumps, err := ListDumps()
	if err != nil {
		return err
	}

	for _, dump := range dumps {
		err = dump.Remove()
		if err != nil {
			return err
		}
	}

	return nil
}
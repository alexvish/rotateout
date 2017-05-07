package main

import (
	"os"
	"io"
	"time"
	"github.com/lestrrat/go-strftime"
	"github.com/pkg/errors"
	"fmt"
	"github.com/jessevdk/go-flags"
	"math"
	"bytes"
	"regexp"
	"strconv"
	"path/filepath"
)

func check(e error) {
	if e != nil {
		panic(e);
	}
}

type rotation struct {
	startTime    time.Time
	bytesWritten int64
}

type rotateOut struct {
	baseName        string
	suffix          string
	timeFormat      *strftime.Strftime
	maxSize         int64
	maxTime         time.Duration
	useUTC          bool
	rotation        *rotation
	outFile         *os.File
}

type RotateOut interface {
	Run()
}

func (ro *rotateOut) LogFileName() string {
	return ro.baseName + ro.suffix;
}

func (ro *rotateOut) OpenLogFile() {
	fileName := ro.LogFileName();
	out, err := os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	check(err);
	ro.outFile = out;
}

func (ro *rotateOut) Close() {
	if ro.outFile != nil {
		ro.outFile.Close();
		ro.outFile = nil;
	}
}

func (ro *rotateOut) Run() {
	buf := make([]byte, 4096);
	ro.OpenLogFile();
	defer ro.Close()

	ro.UpdateRotation()
	for {
		if (ro.NeedRotate()) {
			ro.Rotate();
		}
		len, err := os.Stdin.Read(buf)
		if err == io.EOF {
			break;
		} else {
			check(err);
		}
		ro.outFile.Write(buf[0:len]);
		ro.rotation.bytesWritten += int64(len);
	}
}

func (ro *rotateOut) NeedRotate() bool {
	if ro.rotation == nil {
		ro.UpdateRotation()
		return false;
	}
	if ro.maxTime > 0 {
		rotDuration := time.Since(ro.rotation.startTime);
		if rotDuration > ro.maxTime {
			return true;
		}
	}

	if ro.maxSize > 0  && ro.rotation.bytesWritten > ro.maxSize {
		return true;
	}
	return false;
}

func (ro *rotateOut) UpdateRotation() {
	r := rotation{
		startTime:    time.Now(),
		bytesWritten: 0,
	}
	ro.rotation = &r;
}

func (ro *rotateOut) NextRotateFilename() string {
	// cat <logBaseName>* should return log records in correct order
	var timePart, sep = "", "."
	if ro.timeFormat != nil {
		t := time.Now();
		if ro.useUTC {
			t = t.UTC();
		}
		timePart = ro.timeFormat.FormatString(t)
		if len(timePart) > 0 {
			timePart = sep + timePart;
			sep = "_";
		}
	}
	var filename = ro.baseName + timePart + ro.suffix;
	for i := 1; i < 10000; i++ {
		_, err := os.Stat(filename)
		if os.IsNotExist(err) {
			break
		} else if err != nil {
			panic(err);
		}
		filename = ro.baseName + timePart + fmt.Sprintf(sep + "%04d", i) + ro.suffix
	}
	return filename
}

func (ro *rotateOut) Rotate() {
	ro.Close()
	err := os.Rename(ro.LogFileName(),ro.NextRotateFilename())
	check(err)
	ro.UpdateRotation()
	ro.OpenLogFile()
}


type optTimeFormat struct {
	Format *strftime.Strftime
}

type optDuration struct {
	Duration time.Duration
}

type optSize struct {
	Size int64
}

type optBaseName struct {
	FileName string
}

type Options struct {
	Format optTimeFormat `short:"f" long:"format" description:"stftime format of time suffix of rotated file: Example: %Y-%m-%d-%H_%M_%S"`
	Utc bool `long:"utc" description:"If set utc time will be used to format rotation timestamps"`
	Ext string `short:"e" long:"ext" description:"Log file extension" default:".log"`
}

type RotateOptions struct {
	MaxDuration optDuration `short:"t" long:"time" description:"Duration between log rotaitons. Value is a sequence of decimal numbers with optional fraction followed by unit. Default unnit is s (second) Walid units are \"w\", \"d\", \"h\", \"m\", \"s\". Example: 3d12.5h" default:"1w"`
	MaxSize optSize `short:"s" long:"size" description:"Size of current file that triggers log rotations. Value is a sequence of decimal numbers with optional fraction followed by unit. Default unnit is b (byte) Walid units are \"g\", \"m\", \"k\", \"b\". Example: 2m512.15k100b" default:"1g"`
	BaseName struct {
		BaseName optBaseName `positional-arg-name:"logBaseName" required:"true"`
	} `positional-args:"true"`
}

func (t *optTimeFormat) UnmarshalFlag(value string) error {
	format, err := strftime.New(value)
	if err != nil {
		return err;
	}
	t.Format = format;
	return nil;
}

func (t *optTimeFormat) MarshalFlag() (string, error) {
	if t.Format != nil {
		return t.Format.Pattern(), nil
	} else {
		return "", nil
	}
}

func (t *optBaseName) UnmarshalFlag(value string) error {
	stat, err := os.Stat(value)
	if err == nil {
		if stat.IsDir() {
			return errors.Errorf("%s - cannot be a directory", value)
		}
	}
	dir := filepath.Dir(value)
	stat,err = os.Stat(dir);
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Errorf("Directory %s does not exist", dir)
		} else {
			return err;
		}
	}
	if err = checkDirWritable(dir); err != nil {
		return errors.Errorf("Directory %s is not writable", dir);
	}
	if !stat.IsDir() {
		return errors.Errorf("%s is not a directory", dir)
	}

	t.FileName = value
	return nil
}

var durationMap = map[string]int64{
	"s": int64(time.Second),
	"S": int64(time.Second),
	"m": int64(time.Minute),
	"M": int64(time.Minute),
	"h": int64(time.Hour),
	"H": int64(time.Hour),
	"d": int64(24*time.Hour),
	"D": int64(24*time.Hour),
	"w": int64(7*24*time.Hour),
	"W": int64(7*24*time.Hour),
}

func (o *optDuration) UnmarshalFlag(value string) error {
	d, err := ParseUnitNum(value,&durationMap,"s")
	if err != nil {
		return err
	}
	o.Duration = time.Duration(d);
	return nil;
}

var sizeMap = map[string]int64{
	"b": 1,
	"B": 1,
	"k": 1024,
	"K": 1024,
	"m": 1024 * 1024,
	"M": 1024 * 1024,
	"g": 1024 * 1024 * 1024,
	"G": 1024 * 1024 * 1024,
}

func (o *optSize) UnmarshalFlag(value string) error {
	s, err := ParseUnitNum(value,&sizeMap,"b")
	if err != nil {
		return err
	}
	o.Size = s;
	return nil;
}


func UnitNumParserError(s, orig, message string, offset int) error {
	var buf bytes.Buffer
	buf.WriteString(message)
	buf.WriteString("\n")
	buf.WriteString(orig)
	buf.WriteString("\n")
	srunes := []rune(s)
	origrunes := []rune(orig);
	for i, l := 0, len(origrunes) - len(srunes) + offset; i < l; i++ {
		buf.WriteString(" ")
	}
	buf.WriteString("^\n");
	return errors.New(buf.String())
}

var numWithUnitRegExp = regexp.MustCompile(`(?P<n>(?:[0-9]*)(?:\.[0-9]+)?)(?P<u>[a-zA-Z]*)`);

func NumWithUnit(val string) (string, float64, string, int, error) {
	res:=numWithUnitRegExp.FindStringSubmatchIndex(val)
	if res == nil || res[0] != 0 {
		return "", 0, "", 0, errors.New("Unexpected character")
	}
	match := val[res[0]:res[1]]
	n := val[res[2]:res[3]]
	u := val[res[4]:res[5]]
	if len(n) == 0 {
		return "", 0, "", 0, errors.New("Number expected")
	}
	nf, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return "", 0, "", 0, errors.New("Cannot convert number")
	}
	return match, nf, u, res[4], nil
}

func ParseUnitNum(s string, unitMap *map[string]int64, defaultUnit string) (int64,error) {
	orig := s
	var result int64 = 0
	if (s == "0") {
		return 0, nil;
	}

	for s != "" {
		match, n, u, unitOffset, err := NumWithUnit(s);
		if err != nil {
			return 0, UnitNumParserError(s,orig, err.Error(), 0)
		}
		if u == "" {
			u = defaultUnit
		}
		unit, ok := (*unitMap)[u]
		if !ok {
			units := make([]string,0, len(*unitMap))
			for k := range *unitMap {
				units = append(units, k);
			}
			return 0, UnitNumParserError(s, orig, fmt.Sprintf("Bad unit: only %v allowed", units), unitOffset)
		}
		if n >= math.MaxInt64/float64(unit)  {
			return 0, UnitNumParserError(s, orig, "Number is too big", 0)
		}
		add := int64(n * float64(unit))
		if (1<<63 - 1) - result < add {
			return 0, UnitNumParserError(s, orig, "Numeric overflow", 0)
		}
		result += add
		s = s[len(match):]
	}
	return result,nil
}



func main() {
	var options Options
	var rotateOptions RotateOptions
	optParser := flags.NewNamedParser("<command>|rotateout", flags.Default | flags.PassAfterNonOption)
	_ , err := optParser.AddGroup("Options", "", &options)
	if err != nil {
		panic(err)
	}
	_ , err = optParser.AddGroup("Rotation Options", "", &rotateOptions)
	if err != nil {
		panic(err)
	}
	_, err = optParser.Parse()
	if err != nil {
		os.Exit(2)
	}
	rotateOut := rotateOut{
		baseName: rotateOptions.BaseName.BaseName.FileName,
		suffix: options.Ext,
		useUTC: options.Utc,
		timeFormat: options.Format.Format,
		maxTime: rotateOptions.MaxDuration.Duration,
		maxSize: rotateOptions.MaxSize.Size,

	}
	rotateOut.Run();
}

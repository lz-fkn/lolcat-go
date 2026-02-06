// lolcat.go - Rainbow text colorizer for terminals
// Port of https://github.com/busyloop/lolcat
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/pflag"
)

const version = "1.1.0"
const origVersion = "100.0.1"

type options struct {
	spread    float64
	freq      float64
	seed      int
	animate   bool
	duration  float64
	speed     float64
	invert    bool
	truecolor bool
	force     bool
	help      bool
	version   bool
	os        float64
}

func main() {
	opts := parseFlags()

	if opts.version {
		fmt.Printf("lolcat %s (c)2011 moe@busyloop.net\n", origVersion)
		fmt.Printf("lolcat-go %s (c)2026 fractus.lol@proton.me\n", version)
		os.Exit(0)
	}

	if opts.help {
		var buf bytes.Buffer
		
		buf.WriteString("Usage: lolcat [OPTION]... [FILE]...\n\n")
		buf.WriteString("Concatenate FILE(s), or standard input, to standard output.\n")
		buf.WriteString("With no FILE, or when FILE is -, read standard input.\n\n")
		
		buf.WriteString("  -a, --animate          Enable psychedelics\n")
		buf.WriteString("  -d, --duration float   Animation duration (default 12)\n")
		buf.WriteString("  -f, --force            Force color even when stdout is not a tty\n")
		buf.WriteString("  -F, --freq float       Rainbow frequency (default 0.1)\n")
		buf.WriteString("  -h, --help             Show this message\n")
		buf.WriteString("  -i, --invert           Invert fg and bg\n")
		buf.WriteString("  -S, --seed int         Rainbow seed, 0 = random\n")
		buf.WriteString("  -s, --speed float      Animation speed (default 20)\n")
		buf.WriteString("  -p, --spread float     Rainbow spread (default 3)\n")
		buf.WriteString("  -t, --truecolor        24-bit (truecolor)\n")
		buf.WriteString("  -v, --version          Print version and exit\n\n")
		
		buf.WriteString("Examples:\n")
		buf.WriteString("  lolcat f - g      Output f's contents, then stdin, then g's contents.\n")
		buf.WriteString("  lolcat            Copy standard input to standard output.\n")
		buf.WriteString("  fortune | lolcat  Display a rainbow cookie.\n\n")
		buf.WriteString("ORIGINAL lolcat's home page: <https://github.com/busyloop/lolcat/>\n")
		buf.WriteString("Ported lolcat's home page: <https://github.com/lz-fkn/lolcat-go/>\n")

		opts.os = float64(rand.Intn(256))
		
		lines := strings.Split(buf.String(), "\n")
		for _, line := range lines {
			opts.os++
			printLine(line, opts, false)
			fmt.Println()
		}
		os.Exit(0)
	}

	if opts.spread < 0.1 {
		fmt.Fprintf(os.Stderr, "lolcat: spread must be >= 0.1\n")
		os.Exit(1)
	}
	if opts.duration < 0.1 {
		fmt.Fprintf(os.Stderr, "lolcat: duration must be >= 0.1\n")
		os.Exit(1)
	}
	if opts.speed < 0.1 {
		fmt.Fprintf(os.Stderr, "lolcat: speed must be >= 0.1\n")
		os.Exit(1)
	}

	opts.os = float64(opts.seed)
	if opts.seed == 0 {
		opts.os = float64(rand.Intn(256))
	}

	files := pflag.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	if opts.animate && isTTY(os.Stdout) {
		fmt.Print("\x1b[?25l")
	}

	exitCode := 0

	for _, filename := range files {
		var fd *os.File
		var err error

		if filename == "-" {
			fd = os.Stdin
		} else {
			fd, err = os.Open(filename)
			if err != nil {
				handleFileError(filename, err)
				exitCode = 1
				continue
			}
			defer fd.Close()
		}

		err = processFile(fd, opts)
		if err != nil {
			if strings.Contains(err.Error(), "broken pipe") {
				break
			}
			handleFileError(filename, err)
			exitCode = 1
		}
	}

	if isTTY(os.Stdout) {
		fmt.Print("\x1b[m\x1b[?25h\x1b[?1;5;2004l")
	}

	os.Exit(exitCode)
}

func parseFlags() options {
	var opts options

	pflag.Float64VarP(&opts.spread, "spread", "p", 3.0, "Rainbow spread")
	pflag.Float64VarP(&opts.freq, "freq", "F", 0.1, "Rainbow frequency")
	pflag.IntVarP(&opts.seed, "seed", "S", 0, "Rainbow seed, 0 = random")
	pflag.BoolVarP(&opts.animate, "animate", "a", false, "Enable psychedelics")
	pflag.Float64VarP(&opts.duration, "duration", "d", 12, "Animation duration")
	pflag.Float64VarP(&opts.speed, "speed", "s", 20.0, "Animation speed")
	pflag.BoolVarP(&opts.invert, "invert", "i", false, "Invert fg and bg")
	pflag.BoolVarP(&opts.truecolor, "truecolor", "t", false, "24-bit (truecolor)")
	pflag.BoolVarP(&opts.force, "force", "f", false, "Force color even when stdout is not a tty")
	pflag.BoolVarP(&opts.help, "help", "h", false, "Show this message")
	pflag.BoolVarP(&opts.version, "version", "v", false, "Print version and exit")

	pflag.Parse()
	return opts
}

func handleFileError(filename string, err error) {
	switch {
	case os.IsNotExist(err):
		fmt.Fprintf(os.Stderr, "lolcat: %s: No such file or directory\n", filename)
	case os.IsPermission(err):
		fmt.Fprintf(os.Stderr, "lolcat: %s: Permission denied\n", filename)
	default:
		if strings.Contains(err.Error(), "is a directory") {
			fmt.Fprintf(os.Stderr, "lolcat: %s: Is a directory\n", filename)
		} else if strings.Contains(err.Error(), "not a regular file") || strings.Contains(err.Error(), "no such device") {
			fmt.Fprintf(os.Stderr, "lolcat: %s: Is not a regular file\n", filename)
		} else {
			fmt.Fprintf(os.Stderr, "lolcat: %s: %v\n", filename, err)
		}
	}
}

func processFile(fd *os.File, opts options) error {
	if !isTTY(os.Stdout) && !opts.force {
		if isTTY(fd) {
			scanner := bufio.NewScanner(fd)
			for scanner.Scan() {
				fmt.Println(scanner.Text())
			}
			return scanner.Err()
		}
		_, err := io.Copy(os.Stdout, fd)
		return err
	}

	reader := bufio.NewReaderSize(fd, 4096)

	for {
		line, err := readLine(reader)
		if err != nil && err != io.EOF {
			return err
		}
		if len(line) == 0 && err == io.EOF {
			break
		}

		opts.os++

		chomped := len(line) > 0 && line[len(line)-1] == '\n'
		if chomped {
			line = line[:len(line)-1]
		}
		line = strings.ReplaceAll(line, "\t", "        ")

		if opts.animate {
			animateLine(line, &opts, chomped)
		} else {
			newOs := printLine(line, opts, chomped)
			if chomped {
				fmt.Println()
			}
			opts.os = newOs
		}

		if err == io.EOF {
			break
		}
	}

	return nil
}

func readLine(reader *bufio.Reader) (string, error) {
	var buf strings.Builder
	for {
		chunk, err := reader.ReadString('\n')
		buf.WriteString(chunk)
		if err != nil {
			if err == io.EOF {
				return buf.String(), err
			}
			return buf.String(), err
		}
		if strings.HasSuffix(chunk, "\n") {
			break
		}
	}
	return buf.String(), nil
}

func animateLine(line string, opts *options, chomped bool) {
	if len(line) == 0 {
		if chomped {
			fmt.Println()
		}
		return
	}

	realOs := opts.os
	stripped := stripAnimationUnfriendlyCodes(line)

	fmt.Print("\x1b7")

	for i := 1; i <= int(opts.duration); i++ {
		fmt.Print("\x1b8")
		
		opts.os = realOs + float64(i)*opts.spread
		
		printLine(stripped, *opts, false)
		
		time.Sleep(time.Duration(1000.0/opts.speed*1000000) * time.Nanosecond)
	}
	
	if chomped {
		fmt.Println()
	} else {
		fmt.Println()
	}
	
	opts.os = realOs
}

func stripAnimationUnfriendlyCodes(line string) string {
	var result strings.Builder
	i := 0
	for i < len(line) {
		if i < len(line)-1 && line[i] == '\x1b' && line[i+1] == '[' {
			start := i
			i += 2
			for i < len(line) && ((line[i] >= '0' && line[i] <= '?') || line[i] == ';' || line[i] == ':') {
				i++
			}
			if i < len(line) {
				cmd := line[i]
				i++
				if cmd != '@' && cmd != 'J' && cmd != 'K' && cmd != 'P' && cmd != 'X' {
					result.WriteString(line[start:i])
				}
			} else {
				result.WriteString(line[start:])
			}
		} else {
			result.WriteByte(line[i])
			i++
		}
	}
	return result.String()
}

type ansiMatch struct {
	escape string
	char   string
}

func printLine(str string, opts options, chomped bool) float64 {
	matches := scanAnsiEscapes(str)
	
	for i, match := range matches {
		if match.char == "" {
			fmt.Print(match.escape)
			continue
		}

		colorR, colorG, colorB := rainbow(opts.freq, opts.os+float64(i)/opts.spread)
		
		if opts.invert {
			fmt.Printf("%s\x1b[48;2;%d;%d;%dm%s\x1b[49m", match.escape, colorR, colorG, colorB, match.char)
		} else {
			fmt.Printf("%s\x1b[38;2;%d;%d;%dm%s\x1b[39m", match.escape, colorR, colorG, colorB, match.char)
		}
	}

	if !chomped {
		return opts.os + float64(len(matches))/opts.spread
	}
	return opts.os
}

func scanAnsiEscapes(s string) []ansiMatch {
	var result []ansiMatch
	i := 0
	
	for i < len(s) {
		if s[i] == '\x1b' {
			escapeStart := i
			i++
			
			if i >= len(s) {
				result = append(result, ansiMatch{escape: s[escapeStart:], char: ""})
				break
			}

			extractChar := func(currentIdx int) (string, int) {
				if currentIdx >= len(s) {
					return "", currentIdx
				}
				if s[currentIdx] == '\x1b' {
					return "", currentIdx
				}
				r, size := utf8.DecodeRuneInString(s[currentIdx:])
				return string(r), currentIdx + size
			}

			switch s[i] {
			case '[':
				i++
				for i < len(s) {
					c := s[i]
					if (c >= '0' && c <= '?') || c == ';' {
						i++
					} else {
						break
					}
				}
				for i < len(s) && s[i] >= ' ' && s[i] <= '/' {
					i++
				}
				if i < len(s) && s[i] >= '@' && s[i] <= '~' {
					i++
				}
				
				escape := s[escapeStart:i]
				char, newI := extractChar(i)
				i = newI
				result = append(result, ansiMatch{escape: escape, char: char})
				continue

			case ']', 'P', 'X', '^', '_':
				i++
				for i < len(s) && s[i] != '\a' && s[i] != '\x1b' {
					i++
				}
				if i < len(s) {
					if s[i] == '\a' {
						i++
					} else if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
						i += 2
					}
				}
				
				escape := s[escapeStart:i]
				char, newI := extractChar(i)
				i = newI
				result = append(result, ansiMatch{escape: escape, char: char})
				continue

			default:
				if (s[i] >= ' ' && s[i] <= '/') || (s[i] >= '@' && s[i] <= '_') {
					i++
				}
				
				escape := s[escapeStart:i]
				char, newI := extractChar(i)
				i = newI
				result = append(result, ansiMatch{escape: escape, char: char})
				continue
			}
		} else {
			r, size := utf8.DecodeRuneInString(s[i:])
			char := string(r)
			i += size
			result = append(result, ansiMatch{escape: "", char: char})
		}
	}
	
	return result
}

func rainbow(freq, i float64) (r, g, b int) {
	red := math.Sin(freq*i+0) * 127 + 128
	green := math.Sin(freq*i+2*math.Pi/3) * 127 + 128
	blue := math.Sin(freq*i+4*math.Pi/3) * 127 + 128
	return int(red), int(green), int(blue)
}

func isTrueColorTerminal() bool {
	term := os.Getenv("COLORTERM")
	return term == "truecolor" || term == "24bit"
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
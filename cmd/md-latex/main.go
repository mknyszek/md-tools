package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	flagIn      = flag.String("i", "", "input file (default: stdin)")
	flagOut     = flag.String("o", "", "output file (default: stdout)")
	flagImgDir  = flag.String("img-dir", "", "directory to generate images to (default: PWD)")
	flagCvtPath = flag.String("tex2svg", "", "location of tex2svg utility (default: same directory as binary)")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	inFile := os.Stdin
	outFile := os.Stdout

	var outFileDir string
	var imgDir string
	var err error
	if inPath := *flagIn; inPath != "" {
		inFile, err = os.Open(inPath)
		if err != nil {
			return err
		}
		defer inFile.Close()
	}
	if imgDir = *flagImgDir; imgDir != "" {
		imgDir, err = filepath.Abs(imgDir)
		if err != nil {
			return err
		}
	} else {
		imgDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	if outPath := *flagOut; outPath != "" {
		outFile, err = os.Create(outPath)
		if err != nil {
			return err
		}
		defer outFile.Close()
		outFileDir, err = filepath.Abs(filepath.Dir(outPath))
		if err != nil {
			return err
		}
	} else {
		// So that Rel deeper down doesn't change anything.
		outFileDir = imgDir
	}
	if err := os.MkdirAll(imgDir, 0o777); err != nil {
		return err
	}

	// Eat up all of the innput.
	b, err := ioutil.ReadAll(inFile)
	if err != nil {
		return err
	}

	return process(bytes.NewReader(b), outFile, outFileDir, imgDir)
}

var inlineLatexExp = regexp.MustCompile("`\\$[^\\$]*\\$`")

func process(in io.Reader, out io.Writer, outFileDir, imgDir string) error {
	s := bufio.NewScanner(in)
	consumeEqn := false
	var mathBuf strings.Builder
	for s.Scan() {
		line := s.Text()
		trimmedLine := strings.TrimSpace(line)
		if consumeEqn {
			if trimmedLine == "```" {
				eqName, outRel, err := createSVG(mathBuf.String(), outFileDir, imgDir, false)
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "![%s](%s)\n", eqName, outRel)
				mathBuf.Reset()
				consumeEqn = false
			} else {
				mathBuf.WriteString(line)
				mathBuf.WriteString("\n")
			}
		} else {
			if trimmedLine == "```render-latex" {
				consumeEqn = true
			} else if matches := inlineLatexExp.FindAllStringIndex(line, -1); len(matches) > 0 {
				var newLine strings.Builder
				lastIdx := 0
				for _, rng := range matches {
					newLine.WriteString(line[lastIdx:rng[0]])
					eqName, outRel, err := createSVG(line[rng[0]+2:rng[1]-2], outFileDir, imgDir, true)
					if err != nil {
						return err
					}
					imgRef := fmt.Sprintf("![%s](%s)", eqName, outRel)
					newLine.WriteString(imgRef)
					lastIdx = rng[1]
				}
				newLine.WriteString(line[lastIdx:])
				fmt.Fprintln(out, newLine.String())
			} else {
				fmt.Fprintln(out, line)
			}
		}
	}
	if err := s.Err(); err != nil {
		return err
	}
	return nil
}

var (
	svgInlineCache = make(map[string]string)
	svgNumInline   = 1
	svgNumEqn      = 1
)

func createSVG(eq, outFileDir, outDir string, inline bool) (string, string, error) {
	var name, fname string
	if inline {
		name = fmt.Sprintf("`%s`", eq)
		if cached, ok := svgInlineCache[eq]; ok {
			return name, cached, nil
		}
		fname = fmt.Sprintf("inl%d.svg", svgNumInline)
		svgNumInline++
	} else {
		name = fmt.Sprintf("Equation %d", svgNumEqn)
		fname = fmt.Sprintf("eqn%d.svg", svgNumEqn)
		svgNumEqn++
	}
	imgOutPath := filepath.Join(outDir, fname)
	imgOut, err := os.Create(imgOutPath)
	if err != nil {
		return "", "", err
	}
	if err := genEqSVG(eq, imgOut, inline); err != nil {
		imgOut.Close()
		return "", "", err
	}
	imgOut.Close()
	outRel, err := filepath.Rel(outFileDir, imgOutPath)
	if err != nil {
		return "", "", err
	}
	if inline {
		svgInlineCache[eq] = outRel
	}
	return name, outRel, nil
}

func genEqSVG(eq string, out io.Writer, inline bool) error {
	cvtPath := *flagCvtPath
	if cvtPath == "" {
		cvtPath = filepath.Join(filepath.Dir(os.Args[0]), "tex2svg")
	}
	cmd := exec.Command(
		*flagCvtPath,
		fmt.Sprintf("--inline=%t", inline),
		eq,
	)
	cmd.Stdout = out
	return cmd.Run()
}

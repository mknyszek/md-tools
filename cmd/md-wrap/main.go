package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// countQuoteDepth looks over the input string, which must be a line
// not containing any newlines (\r?\n), and counts how deeply quoted
// the line is in markdown formatting. It returns this depth and
// the amount of bytes the quote prefix uses as len. If there is whitespace
// after the last quote character but before the content begins,
// then len includes that space.
func countQuoteDepth(line string) (depth, len int) {
	bytes := 0
	consumedSpaceAfter := true
	for _, r := range []rune(line) {
		if unicode.IsSpace(r) {
			bytes += utf8.RuneLen(r)
			if !consumedSpaceAfter {
				len = bytes
				consumedSpaceAfter = true
			}
		} else if r == '>' {
			bytes++
			len = bytes
			consumedSpaceAfter = false
			depth++
		} else {
			break
		}
	}
	return
}

type listType int

const (
	noList listType = iota
	numList
	bulletList
)

func (t listType) symbol() string {
	switch t {
	case numList:
		return "1."
	case bulletList:
		return "*"
	}
	return ""
}

func (t listType) runes() int {
	switch t {
	case numList:
		return 2
	case bulletList:
		return 1
	}
	return 0
}

// countListIndent looks over a line and returns whether it
// contains some kind of list, and what the indent of the
// line is, all encapsulated as a listState. It assumes that
// the line contains no newlines (\r?\n) and that it contains
// no markdown quoting.
func countListIndent(line string) (l listState) {
	runes := []rune(line)
	for i, r := range runes {
		if unicode.IsSpace(r) {
			l.indent++
			l.indentBytes += utf8.RuneLen(r)
		} else if unicode.IsDigit(r) {
			if len(runes) <= i+2 {
				return
			}
			if runes[i+1] == '.' && unicode.IsSpace(runes[i+2]) {
				l.typ = numList
			}
			return
		} else if r == '*' {
			l.typ = bulletList
			return
		} else {
			return
		}
	}
	return
}

func endsSentence(word string) bool {
	return (strings.HasSuffix(word, ".") ||
		strings.HasSuffix(word, ".\"") ||
		strings.HasSuffix(word, ".'")) &&
		!strings.HasSuffix(word, "e.g.") &&
		!strings.HasSuffix(word, "vs.") &&
		!strings.HasSuffix(word, "i.e.")
}

type listState struct {
	typ         listType
	indent      int
	indentBytes int
}

type fmtState struct {
	charsPerLine     int
	newLine          strings.Builder
	newLineRunes     int
	inCode           bool
	list             listState
	appliedFirstList bool
	listPrefixFirst  string
	listPrefixRest   string
	out              io.Writer
}

func newFmtState(charsPerLine int, out io.Writer) *fmtState {
	return &fmtState{charsPerLine: charsPerLine, out: out}
}

// setListState updates the current running list state.
func (f *fmtState) setListState(l listState) {
	f.list = l
	if l.typ != noList {
		f.appliedFirstList = false
		f.listPrefixFirst = strings.Repeat(" ", l.indent) + l.typ.symbol() + " "
		f.listPrefixRest = strings.Repeat(" ", l.indent+l.typ.runes()+1)
	} else {
		f.listPrefixFirst = ""
		f.listPrefixRest = ""
	}
}

func (f *fmtState) writeToLine(s string) {
	f.newLine.WriteString(s)
	f.newLineRunes += len([]rune(s))
}

func (f *fmtState) flushLine() {
	fmt.Fprintln(f.out, strings.TrimRightFunc(f.newLine.String(), unicode.IsSpace))
	f.newLineRunes = 0
	f.newLine.Reset()
}

func (f *fmtState) process(in io.Reader) {
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		line := s.Text()
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "```") {
			// Check if we're entering or exiting a code block.
			if !f.inCode {
				if f.newLineRunes != 0 {
					f.flushLine()
				}
				f.setListState(listState{})
			}
			f.inCode = !f.inCode
			f.writeToLine(trimmedLine)
			f.flushLine()
			continue
		}
		if len(trimmedLine) == 0 {
			if f.newLineRunes != 0 {
				f.flushLine()
			}
			f.setListState(listState{})
			f.flushLine()
			continue
		}
		if f.inCode {
			// Leave code lines alone.
			f.writeToLine(line)
			f.flushLine()
			continue
		}

		quoteDepth, quoteLen := countQuoteDepth(line)
		quotePrefix := strings.Repeat("> ", quoteDepth)
		line = line[quoteLen:]

		newList := countListIndent(line)
		if newList.typ != noList {
			if f.newLineRunes != 0 {
				f.flushLine()
			}
			f.setListState(newList)
		} else if f.list.typ != noList && newList.indent != f.list.indent+f.list.typ.runes()+1 {
			if f.newLineRunes != 0 {
				f.flushLine()
			}
			f.setListState(listState{})
		}
		var listPrefix string
		if f.list.typ != noList {
			if newList.typ != noList {
				listPrefix = f.listPrefixFirst
			} else {
				listPrefix = f.listPrefixRest
			}
		}
		line = line[f.list.indentBytes+len(f.list.typ.symbol()):]

		sw := bufio.NewScanner(strings.NewReader(line))
		sw.Split(bufio.ScanWords)
		for sw.Scan() {
			word := sw.Text()
			if f.newLineRunes != 0 && f.newLineRunes+len([]rune(word)) > f.charsPerLine {
				f.flushLine()
			}
			if f.newLineRunes == 0 {
				f.writeToLine(quotePrefix)
				f.writeToLine(listPrefix)
				if !f.appliedFirstList {
					f.appliedFirstList = true
					listPrefix = f.listPrefixRest
				}
			}
			f.writeToLine(word)
			if endsSentence(word) {
				f.flushLine()
			} else {
				f.writeToLine(" ")
			}
		}
	}
	if f.newLineRunes != 0 {
		f.flushLine()
	}
}

func main() {
	fs := newFmtState(80, os.Stdout)
	fs.process(os.Stdin)
}

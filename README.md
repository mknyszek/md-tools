# Markdown tools

This repository contains miscellaneous useful tools for markdown documentation.

## md-wrap

This tool wraps a markdown document to 80 characters and also tries to put new
sentences on a new line, preserving quote blocks (`>`), lists, and skipping over
verbatim blocks.

This tool processes STDIN and writes to STDOUT.

This tool only requires Go.

## md-latex

This tool processes LaTeX embedded in the markdown document, generates SVG files
for each one, and embeds links to those SVGs from within the document.
It uses MathJax to generate SVGs, which only supports the math-oriented subset
of LaTeX.

This tool requires:
- Go
- node.js

Unfortunately this tool isn't easy to move around because it needs to reference
a pile of Javascript, so I recommend invoking this tool via

```
go run ./cmd/md-latex -tex2svg ./cmd/md-latex/tex2svg
```

for the time being.

The tool understands the following patterns.

For out-of-line LaTeX:

	```render-latex
	\frac{x}{y}
	```

For in-line LaTeX:

	`$\frac{x}{y}$`


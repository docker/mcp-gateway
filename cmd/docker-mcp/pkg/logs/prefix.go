package logs

import (
	"bytes"
	"io"
)

type prefixer struct {
	prefix          string
	writer          io.Writer
	trailingNewline bool
	buf             bytes.Buffer
}

func NewPrefixer(writer io.Writer, prefix string) io.Writer {
	return &prefixer{
		prefix:          prefix,
		writer:          writer,
		trailingNewline: true,
	}
}

func (pf *prefixer) Write(payload []byte) (int, error) {
	pf.buf.Reset()

	for _, b := range payload {
		if pf.trailingNewline {
			pf.buf.WriteString(pf.prefix)
			pf.trailingNewline = false
		}

		pf.buf.WriteByte(b)

		if b == '\n' {
			pf.trailingNewline = true
		}
	}

	n, err := pf.writer.Write(pf.buf.Bytes())
	if err != nil {
		if n > len(payload) {
			n = len(payload)
		}
		return n, err
	}

	return len(payload), nil
}

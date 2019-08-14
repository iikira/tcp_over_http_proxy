package lineconfig

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/iikira/BaiduPCS-Go/pcsliner/args"
	"github.com/iikira/BaiduPCS-Go/pcsutil/converter"
	"io"
	"os"
	"strconv"
	"strings"
)

type (
	LineConfig map[string]string
)

var (
	ErrSyntax = errors.New("syntax error")
)

func NewLineConfig() LineConfig {
	return LineConfig{}
}

func (lc LineConfig) lazyInit() {
	if lc == nil {
		lc = LineConfig{}
	}
}

func (lc LineConfig) LoadFrom(fPath string) (err error) {
	file, err := os.Open(fPath)
	if err != nil {
		return
	}
	defer file.Close()

	err = lc.ParseFromReader(file)
	if err != nil {
		return
	}
	return
}

func (lc LineConfig) ParseFromReader(reader io.Reader) (err error) {
	r := bufio.NewReader(reader)
	for {
		line, err := r.ReadBytes(';')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		line = bytes.TrimSpace(line)
		line = bytes.TrimSuffix(line, []byte{';'})
		if len(line) == 0 {
			continue
		}

		if line[0] == '#' { // 注释
			continue
		}

		splits := bytes.SplitN(line, []byte{'='}, 2)
		if len(splits) != 2 {
			return ErrSyntax
		}

		splits[1] = bytes.TrimFunc(splits[1], args.IsQuote)
		unquote, err := strconv.Unquote("\"" + converter.ToString(splits[1]) + "\"")
		if err != nil {
			return err
		}

		unquote = strings.TrimFunc(unquote, args.IsQuote)
		lc[converter.ToString(splits[0])] = unquote
	}

	return nil
}

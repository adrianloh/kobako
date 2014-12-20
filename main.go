package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"go/format"
	"io"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/alecthomas/kingpin.v1"
)

const CONFIG_FILE = "config.kobako"

const HEAD = `package BOOM

import "encoding/base64"

type KobakoResource struct {
	contentType string
	data        []byte
}

var __nib__ = map[string]map[string]string{
`

const TAIL = `
var Kobako = func() map[string]KobakoResource {
	resource := map[string]KobakoResource{}
	for filename, content := range __nib__ {
		data, err := base64.StdEncoding.DecodeString(content["data"])
		if err == nil {
			resource[filename] = KobakoResource{content["content-type"], data}
		}
	}
	return resource
}()
`

const LINE = `		"PATH": map[string]string{"content-type": "CONTENT_TYPE", "data": "DATA"}`

const (
	EOL_CONTINUE = ",\n"
	EOL_END      = "}\n"
)

func encode(path string) string {
	bb := &bytes.Buffer{}
	wb64 := base64.NewEncoder(base64.StdEncoding, bb)
	wgz, _ := gzip.NewWriterLevel(wb64, gzip.BestCompression)
	f, _ := os.Open(path)
	io.Copy(wgz, f)
	wgz.Close()
	wb64.Close()
	f.Close()
	return bb.String()
}

func getContentType(path string) string {
	c := "application/octet-stream"
	ext := filepath.Ext(path)
	if ext != "" {
		ext = mime.TypeByExtension(ext)
		if ext != "" {
			c = strings.Split(ext, ";")[0]
		}
	}
	return c
}

func makeMapString(key string, path string) *string {
	data := encode(path)
	contentType := getContentType(path)
	nt := strings.Replace(LINE, "PATH", key, 1)
	nt = strings.Replace(nt, "CONTENT_TYPE", contentType, 1)
	nt = strings.Replace(nt, "DATA", data, 1)
	return &nt
}

func loadConfig(root string) (string, []func(string, string, bool) bool) {

	pkg := "main"
	filters := []func(string, string, bool) bool{}

	filters = append(filters, func(path string, filename string, isFile bool) bool {
		return !strings.HasPrefix(filename, ".") && !(filename == CONFIG_FILE)
	})

	configPath := filepath.Join(root, CONFIG_FILE)

	f, err := os.Open(configPath)

	if err == nil {
		scan := bufio.NewScanner(f)
		for scan.Scan() {
			line := scan.Text()
			line = strings.Trim(line, " ")
			if line != "" {
				switch {
				case strings.HasPrefix(line, "#package"):
					pkg = strings.Split(line, " ")[1]
				default:
					re, re_err := regexp.Compile(line)
					if re_err == nil {
						filters = append(filters, func(path string, filename string, isFile bool) bool {
							return !re.MatchString(path)
						})
					} else {
						fmt.Errorf("Invalid match line: %s", line)
					}
				}
			}
		}
	}

	return pkg, filters

}

func main() {

	pwd, _ := os.Getwd()

	var (
		_goout_ = kingpin.Flag("out", "Path of the go file to generate").Short('O').Required().String()
		_root_  = kingpin.Arg("root", "The root directory where files are served").Default(pwd).String()
	)

	kingpin.Parse()

	root := *_root_
	fPath := *_goout_

	buff := &bytes.Buffer{}

	totalFiles := 0
	block := make(chan bool)
	lineChan := make(chan *string)
	process := make(chan string, 1000000)

	mime.AddExtensionType(".woff2", "application/font-woff2")

	pkgName, filters := loadConfig(root)
	head := strings.Replace(HEAD, "BOOM", pkgName, 1)

	buff.WriteString(head)

	go func() {
		done := 0
		for {
			nt := <-lineChan
			buff.WriteString(*nt)
			done++
			if done == totalFiles {
				buff.WriteString(EOL_END)
				block <- false
				break
			} else {
				buff.WriteString(EOL_CONTINUE)
			}
		}
	}()

	exit := make(chan int)

	for i := 0; i < 50; i++ {
		go func(i int) {
			defer func() { exit <- i }()
			for {
				path, ok := <-process
				if !ok {
					break
				}
				key, err := filepath.Rel(root, path)
				if err == nil {
					lineChan <- makeMapString(key, path)
					fmt.Printf("[package %s] %s\n", pkgName, key)
				}
			}
		}(i)
	}

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		filename := info.Name()
		isFile := info.Mode().IsRegular()
		if isFile {
			for i, f := range filters {
				if !f(path, filename, isFile) {
					break
				}
				if i == len(filters)-1 {
					totalFiles++
					process <- path
				}
			}
		}
		return nil
	})

	<-block

	close(process)

	for i := 0; i < 50; i++ {
		<-exit
	}

	buff.WriteString(TAIL)

	formatted, err := format.Source(buff.Bytes())
	if err != nil {
		fmt.Println(err.Error())
		fmt.Errorf("Formatting error. Failed to create go file.")
	} else {
		f, _ := os.Create(fPath)
		f.Write(formatted)
		f.Close()
		fmt.Printf("OK [%d files] Saved to: %s\n", totalFiles, fPath)
	}

}

/*




















*/

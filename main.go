package main

import (
	"archive/zip"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"gopkg.in/alecthomas/kingpin.v2"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const targetTemplate = "https://chromedriver.storage.googleapis.com/%s/chromedriver_win32.zip"

var (
	specVersion string
	outputPath  string
	isShowList  bool
)

func init() {
	kingpin.Flag("version", "specify for major version. for example chrome version is '101.xxx...' then '--version=101'").Short('v').StringVar(&specVersion)
	kingpin.Flag("out", "specify for unzip path.").Short('o').Default(".").StringVar(&outputPath)
	kingpin.Flag("list", "show specifiable chrome driver versions.").Default("false").Short('l').BoolVar(&isShowList)
	kingpin.Parse()
}

func main() {
	if isShowList && strings.EqualFold(specVersion, "") {
		showList()
		return
	}

	_, versions := getChromeVersions(false)
	if version, ok := versions[specVersion]; ok {
		latestVersion := version[0]
		zipFilePath, err, tempClose := downloadZipFile(latestVersion)
		if err != nil {
			panic(err)
		}
		defer tempClose()

		if err := unzip(zipFilePath, outputPath); err != nil {
			panic(err)
		}
	} else {
		panic(fmt.Errorf("can't specify version.\n"))
	}
}

func downloadZipFile(version string) (string, error, func() error) {
	target := fmt.Sprintf(targetTemplate, version)
	baseName := filepath.Base(target)
	resp, err := http.Get(target)
	if err != nil {
		return "", err, nil
	}
	defer resp.Body.Close()

	finFunc, tempPath, err := createTemp(".", time.Now().Format(".2006010215030405"))
	if err != nil {
		return "", err, nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err, finFunc
	}

	zipFilePath := tempPath + string(os.PathSeparator) + baseName
	z, err := os.Create(zipFilePath)
	if err != nil {
		return "", err, finFunc
	}
	defer z.Close()

	if _, err := z.Write(body); err != nil {
		return "", err, finFunc
	}

	return zipFilePath, nil, finFunc
}

func showList() {
	majors, versions := getChromeVersions(false)

	fmt.Println("Specifiable chrome driver versions.")
	fmt.Printf("Major\tLatest\n")
	for _, major := range majors {
		fmt.Printf("%s\t%s\n", major, versions[major][0])
	}
	return
}

func getChromeVersions(isLatest bool) ([]string, map[string][]string) {
	resp, err := http.Get("https://chromedriver.chromium.org/downloads")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		panic(err)
	}

	s := doc.Find(".XqQF9c")

	versionMap := make(map[string][]string)
	loopCnt := s.Size()
	if isLatest {
		loopCnt = 3
	}
	for i := 0; i < loopCnt; i++ {
		for _, attr := range s.Get(i).Attr {
			if strings.EqualFold(attr.Key, "href") {
				if strings.Contains(attr.Val, "https://chromedriver.storage.googleapis.com/index.html?") {
					versions := strings.Split(attr.Val, "=")
					if len(versions) == 2 {
						version := strings.Replace(versions[1], "/", "", -1)
						reg, err := regexp.Compile(`^\d{1,3}`)
						if err != nil {
							panic(err)
						}
						majorVersion := reg.FindString(version)
						versionMap[majorVersion] = append(versionMap[majorVersion], version)
					}
				}
			}
			continue
		}
	}

	var keysInt []int
	for key, _ := range versionMap {
		ki, err := strconv.Atoi(key)
		if err != nil {
			panic(err)
		}
		keysInt = append(keysInt, ki)
		sort.Sort(sort.Reverse(sort.StringSlice(versionMap[key])))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(keysInt)))

	var keys []string
	for _, val := range keysInt {
		keys = append(keys, strconv.Itoa(val))
	}
	return keys, versionMap
}

func createTemp(dir, patterns string) (func() error, string, error) {
	tmp, err := os.MkdirTemp(dir, patterns)
	if err != nil {
		return nil, "", err
	}

	return func() error {
		return os.RemoveAll(tmp)
	}, tmp, nil
}

func unzip(src, dest string) error {
	zipped, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zipped.Close()

	wg := &sync.WaitGroup{}
	for _, zippedFile := range zipped.File {
		wg.Add(1)

		go func() {
			defer func() {
				switch recover() {
				default:
					wg.Done()
				}
			}()
			f, err := zippedFile.Open()
			if err != nil {
				panic(err)
			}
			defer f.Close()

			if zippedFile.FileInfo().IsDir() {
				path := filepath.Join(dest, zippedFile.Name)
				os.MkdirAll(path, zippedFile.Mode())
			} else {
				buf := make([]byte, zippedFile.UncompressedSize64)
				_, err = io.ReadFull(f, buf)
				if err != nil {
					panic(err)
				}

				path := filepath.Join(dest, zippedFile.Name)
				err := ioutil.WriteFile(path, buf, zippedFile.Mode())
				if err != nil {
					panic(err)
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

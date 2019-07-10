// Copyright 2016 Raul Santos Lebrato
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

var notoDirectory = "noto-emoji"

// Emoji contains the data of an emoji
type Emoji struct {
	Unicode string
	SVG     string
}

func updateFile(update bool, path string) bool {
	_, err := os.Stat(path)
	exists := !os.IsNotExist(err)
	if exists && update {
		os.RemoveAll(path)
	}
	if !exists || update {
		return true
	}
	return false
}

func updateNotoEmoji(update bool) {
	if updateFile(update, notoDirectory) {
		fmt.Println("Downloading noto-emoji...")
		exec.Command("git", "clone", "https://github.com/googlei18n/noto-emoji", notoDirectory).Run()
		fmt.Println("noto-emoji finished downloading")
	}
}

func unicodeToEmoji(unicode string, separator string) (string, error) {
	unicodeStr := strings.Split(unicode, separator)
	for index, unicodeChar := range unicodeStr {
		hex, err := strconv.ParseInt(unicodeChar, 16, 64)
		if err != nil {
			return "", err
		}
		unicodeStr[index] = string(hex)
	}
	return strings.Join(unicodeStr, ""), nil
}

func fileToEmoji(filePath string, fileName string) (*Emoji, error) {
	emoji := &Emoji{}

	file, err := os.Open(filePath + fileName)
	if err != nil {
		panic(err)
	}

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}

	regexpNewLine := regexp.MustCompile(`\r?\n`)
	regexpTabs := regexp.MustCompile(`\t`)

	regexpHeader := regexp.MustCompile(".*<svg.*?>")
	regexpFooter := regexp.MustCompile("</svg>")

	code := regexpNewLine.ReplaceAllString(string(bytes), "")
	code = regexpTabs.ReplaceAllString(code, "")
	code = regexpHeader.ReplaceAllString(code, "")
	code = regexpFooter.ReplaceAllString(code, "")

	unicode := fileName[len("emoji_u") : len(fileName)-len(".svg")]
	unicodedEmoji, err := unicodeToEmoji(unicode, "_")
	if err != nil {
		return nil, err
	}
	emoji.Unicode = unicodedEmoji
	emoji.SVG = code

	return emoji, nil
}

func readEmojis() []Emoji {
	emojis := []Emoji{}

	path := notoDirectory + "/svg/"
	files, err := ioutil.ReadDir(path)
	if err != nil {
		panic(err)
	}

	fmt.Println("Parsing ", len(files), " elements...")
	for index, file := range files {
		emoji, err := fileToEmoji(path, file.Name())
		if err != nil {
			fmt.Println(err.Error())
			fmt.Println("Error parsing element ", index+1, " of ", len(files))
			continue
		}
		emojis = append(emojis, *emoji)
		fmt.Println("Parsed element ", index+1, " of ", len(files))
	}

	return emojis
}

func execTemplate(fileName string, data interface{}) {
	t := template.New("t")
	t, err := template.ParseFiles("templates/" + fileName)
	if err != nil {
		panic(err)
	}

	file, err := os.Create(fileName)
	if err != nil {
		panic(err)
	}

	err = t.ExecuteTemplate(file, fileName, data)
	if err != nil {
		panic(err)
	}
}

func writeIconset(data []Emoji) {
	fmt.Println("Writing iconset...")
	execTemplate("noto-emoji-iconset.html", data)
	fmt.Println("noto-emoji-iconset.html file created")
}

// Emojione represents an emoji in Emojione data
type Emojione struct {
	CodePoints struct {
		Base string `json:"base"`
	} `json:"code_points"`
	Shortname string `json:"shortname"`
}

func writeDictionary() {
	fmt.Println("Writing Emoji Dictionary...")
	resp, err := http.Get("https://raw.githubusercontent.com/Ranks/emojione/master/emoji.json")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var emojione map[string]Emojione
	err = json.Unmarshal(data, &emojione)
	if err != nil {
		panic(err)
	}

	emojis := make(map[string]string)

	fmt.Println("Parsing ", len(emojione), " emojis...")

	index := 0
	for _, emoji := range emojione {
		unicodedEmoji, er := unicodeToEmoji(emoji.CodePoints.Base, "-")
		if er != nil {
			fmt.Println(er.Error())
			fmt.Println("Error parsing element ", index+1, " of ", len(emojione))
			continue
		}
		emojis[emoji.Shortname] = unicodedEmoji
		fmt.Println("Parsed element ", index+1, " of ", len(emojione))
		index++
	}

	fmt.Println("Parsing emojis as JSON...")
	jsonData, err := json.Marshal(emojis)
	if err != nil {
		panic(err)
	}

	execTemplate("emoji-dictionary.html", string(jsonData))

	fmt.Println("emoji-dictionary.html file created")
}

func updateDictionary(update bool) {
	if updateFile(update, "emoji-dictionary.html") {
		writeDictionary()
	}
}

func execAnalyze() {
	fmt.Println("Executing: polymer analyze emoji-icon.html > analysis.json")

	cmd := exec.Command("polymer", "analyze", "emoji-icon.html")
	out, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	outFile, err := os.Create("analysis.json")
	if err != nil {
		panic(err)
	}

	writer := bufio.NewWriter(outFile)

	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	go io.Copy(writer, out)
	cmd.Wait()

	writer.Flush()

	fmt.Println("analysis.json file created")
}

func main() {
	updateNoto := flag.Bool("update-noto", false, "update noto emoji repository")
	updateJSON := flag.Bool("update-dictionary", false, "update emoji one dictionary")
	updateAnalysis := flag.Bool("analysis", false, "update analysis.json using polymer-cli analyze")

	flag.Parse()

	updateNotoEmoji(*updateNoto)
	emojis := readEmojis()
	writeIconset(emojis)
	updateDictionary(*updateJSON)
	if *updateAnalysis {
		execAnalyze()
	}
}

package utils

import (
	"encoding/csv"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

func WriteAppend(file, line string) error {
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return err
	}

	return nil
}

func ReadCSVByLines(filePath string, f func(items []string)) error {
	csvfile, err := os.Open(filePath)
	if err != nil {
		return err
	}

	// Parse the file
	reader := csv.NewReader(csvfile)
	for {
		// Read each record from csv
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		f(record)
	}
	return nil
}

func ReadFileAsString(filePath string) string {
	file, _ := os.Open(filePath)
	defer file.Close()
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func ReadFileByLines(filePath string) []string {
	content := ReadFileAsString(filePath)
	return strings.Split(content, "\n")
}

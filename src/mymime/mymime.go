package mymime

import (
    "errors"
    "os"
    "bufio"
    "strings"
    "fmt"
)

var types = make(map[string] string)

func Load(path string) error {
    if path == "" {
        return errors.New("MIME type file path is empty!")
    }

    file, err := os.Open(path)
    if err != nil {
        return err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()

        if len(line) <= 0 || (len(line) > 0 && line[0] == '#') {
            continue
        }
        fields := strings.Fields(line)

        var value string
        for i, v := range fields {
            if i == 0 {
                value = v
            } else {
                types[v] = value
                fmt.Println("ext: ", v, "type: ", value)
            }
        }
    }

    if err := scanner.Err(); err != nil {
        panic(err)
    }

    return err
}

func TypeByExt(ext string) string {
    thetype, ok := types[ext]
    if ok {
        return thetype
    } else {
        return ""
    }
}

func Count() int {
    return len(types)
}

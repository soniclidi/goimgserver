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
        return errors.New("Config path is empty!")
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

        var thevalue string
        for i, v := range fields {
            if i == 0 {
                thevalue = v
            } else {
                types[v] = thevalue
                fmt.Println("ext: ", v, "type: ", thevalue)
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

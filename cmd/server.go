package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
)


func upload(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if r.Method == "GET" {
		t, err := template.ParseFiles("upload.gptl")
		checkErr(err)
		t.Execute(w, nil)
	} else {
		file, handle, err := r.FormFile("file")
		checkErr(err)
		f, err := os.OpenFile("./test/"+handle.Filename, os.O_WRONLY|os.O_CREATE, 0666)
		io.Copy(f, file)
		checkErr(err)
		defer f.Close()
		defer file.Close()
		fmt.Println("upload success")
	}
}

func checkErr(err error) {
	if err != nil {
		err.Error()
	}
}

func main() {
	http.HandleFunc("/upload", upload)
	err := http.ListenAndServe(":8888", nil)
	if err != nil {
		log.Fatal("listenAndServe: ", err)
	}
}

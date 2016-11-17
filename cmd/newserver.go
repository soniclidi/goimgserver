package main


/*
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <string.h>
#include <errno.h>
#include <sys/types.h>
#include <sys/stat.h>
#include "shared_func.h"
#include "tracker_types.h"
#include "tracker_proto.h"
#include "tracker_client.h"
#include "storage_client.h"
#include "storage_client1.h"
#include "client_func.h"
#include "client_global.h"

#cgo CFLAGS: -I/usr/include/fastcommon -I/usr/include/fastdfs
#cgo LDFLAGS: -L/usr/lib64 -lpthread -lfastcommon -lfdfsclient
*/
import "C"

import (
    "fmt"
    "os"
    "log"
    "io"
    "net/http"
    "html/template"
    "github.com/gin-gonic/gin"
)

func main() {
    router := gin.Default()
    html := template.Must(template.ParseFiles("upload.tmpl"))
    router.SetHTMLTemplate(html)

    router.GET("/upload", func(c *gin.Context) {
        c.HTML(http.StatusOK, "upload.tmpl", nil)
    })

    router.POST("/upload", func(c *gin.Context) {
        file, header, err := c.Request.FormFile("file")
        fmt.Println(header.Filename)
        filename := header.Filename
        out, err := os.Create("./test/" + filename)
        if err != nil {
            log.Fatal(err)
        }
        defer out.Close()
        _, err = io.Copy(out, file)
        if err != nil {
            log.Fatal(err)
        }

        result := C.fdfs_client_init("/etc/fdfs/client.conf")
        fmt.Println("fdfs client init result is", int(result))

        c.JSON(http.StatusOK, gin.H{"fileid": "i am a fileid", "key": "i am a key",})
    })

    router.Run(":8080")
}

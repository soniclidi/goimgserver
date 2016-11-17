package main

import (
    "fmt"
    "os"
    "log"
    "io"
    "net/http"
    "github.com/gin-gonic/gin"
    "html/template"
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

        c.JSON(http.StatusOK, gin.H{"fileid": "i am a fileid", "key": "i am a key",})
    })

    router.GET("/test", func(c *gin.Context) {
        c.File("./hehe.png")
    })

    router.Run(":8080")
}

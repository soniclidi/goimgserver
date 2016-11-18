package main


import (
    "fmt"
    "os"
    "log"
    "io"
    "net/http"
    "html/template"
    "github.com/gin-gonic/gin"
    "unsafe"
)

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


char file_id[128];

int fdfs_upload_file(char *conf_filename, char *local_filename)
{
	char group_name[FDFS_GROUP_NAME_MAX_LEN + 1];
	ConnectionInfo *pTrackerServer;
	int result;
	int store_path_index;
	ConnectionInfo storageServer;
	//char file_id[128];

	if ((result=fdfs_client_init(conf_filename)) != 0)
	{
		return result;
	}

	pTrackerServer = tracker_get_connection();
	if (pTrackerServer == NULL)
	{
		fdfs_client_destroy();
		return errno != 0 ? errno : ECONNREFUSED;
	}

	*group_name = '\0';
	if ((result=tracker_query_storage_store(pTrackerServer, \
	                &storageServer, group_name, &store_path_index)) != 0)
	{
		fdfs_client_destroy();
		return result;
	}

	result = storage_upload_by_filename1(pTrackerServer, \
			&storageServer, store_path_index, \
			local_filename, NULL, \
			NULL, 0, group_name, file_id);

	tracker_disconnect_server_ex(pTrackerServer, true);
	fdfs_client_destroy();

	return result;
}

#cgo CFLAGS: -I/usr/include/fastcommon -I/usr/include/fastdfs
#cgo LDFLAGS: -L/usr/lib64 -lpthread -lfastcommon -lfdfsclient
*/
import "C"

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

        confstr := C.CString("/etc/fdfs/client.conf")
        defer C.free(unsafe.Pointer(confstr))
        localstr := C.CString("./test/" + filename)
        defer C.free(unsafe.Pointer(localstr))
        result := int(C.fdfs_upload_file(confstr, localstr))

        if result == 0 {
            fmt.Println("file id is", C.GoString(&C.file_id[0]))
            c.JSON(http.StatusOK, gin.H{"result": "success", "file_id": "i am a file id", "key": "i am a key",})
        } else {
            c.JSON(http.StatusOK, gin.H{"result": "fail",})
        }


    })

    router.Run(":8080")
}

package main


import (
    "fmt"
    "log"
    "net/http"
    "html/template"
    "github.com/gin-gonic/gin"
    "unsafe"
    "io/ioutil"
    "path"
    "strconv"
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


char out_file_id[128];
char *out_file_buffer;

int fdfs_upload_file(char *conf_filename, char *filebuff, int64_t filesize, char *extname)
{
	char group_name[FDFS_GROUP_NAME_MAX_LEN + 1];
	ConnectionInfo *pTrackerServer;
	int result;
	int store_path_index;
	ConnectionInfo storageServer;

	if ((result = fdfs_client_init(conf_filename)) != 0)
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

	//result = storage_upload_by_filename1(pTrackerServer, \
	//		&storageServer, store_path_index, \
	//		local_filename, NULL, \
	//		NULL, 0, group_name, out_file_id);
	result = storage_upload_by_filebuff1(pTrackerServer, \
			&storageServer, store_path_index, \
			filebuff, filesize, extname, \
			NULL, 0, group_name, out_file_id);

	tracker_disconnect_server_ex(pTrackerServer, true);
	fdfs_client_destroy();

	return result;
}


//int fdfs_download_file(char *conf_filename, char *file_id, char **file_buff, int64_t *file_size)
int fdfs_download_file(char *conf_filename, char *file_id, int64_t *file_size)
{
	ConnectionInfo *pTrackerServer;
	int result;
	int64_t file_offset;
	int64_t download_bytes;

	if ((result = fdfs_client_init(conf_filename)) != 0)
	{
		return result;
	}

	pTrackerServer = tracker_get_connection();
	if (pTrackerServer == NULL)
	{
		fdfs_client_destroy();
		return errno != 0 ? errno : ECONNREFUSED;
	}

	file_offset = 0;
	download_bytes = 0;

	result = storage_do_download_file1_ex(pTrackerServer, \
                NULL, FDFS_DOWNLOAD_TO_BUFF, file_id, \
                file_offset, download_bytes, \
                &out_file_buffer, NULL, file_size);

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

        buff,err := ioutil.ReadAll(file)
        if err != nil {
            log.Fatal(err)
        }

        confstr := C.CString("/etc/fdfs/client.conf")
        defer C.free(unsafe.Pointer(confstr))
        extstr := C.CString(path.Ext(filename)[1:])
        defer C.free(unsafe.Pointer(extstr))
        result := int(C.fdfs_upload_file(confstr, (*C.char)(unsafe.Pointer(&buff[0])), C.int64_t(len(buff)), extstr))

        if result == 0 {
            file_id := C.GoString(&C.out_file_id[0])
            fmt.Println("file id is", file_id)
            c.JSON(http.StatusOK, gin.H{"result": "success", "file_id": file_id, "key": "i am a key",})
        } else {
            c.JSON(http.StatusOK, gin.H{"result": "fail",})
        }

    })

    router.GET("/getimage", func(c *gin.Context) {
        file_id := c.Query("fileid")
        fileidstr := C.CString(file_id)
        defer C.free(unsafe.Pointer(fileidstr))
        confstr := C.CString("/etc/fdfs/client.conf")
        defer C.free(unsafe.Pointer(confstr))

        var file_length C.int64_t

        result := int(C.fdfs_download_file(confstr, fileidstr, &file_length))
        defer C.free(unsafe.Pointer(C.out_file_buffer))

        if result == 0 {
            file_len := int(file_length)
            c.Header("Content-Type", "image/" + path.Ext(file_id)[1:])
            c.Header("Content-Length", strconv.Itoa(file_len))
            fmt.Println("file id is", file_id)
            fmt.Println("file content type is", path.Ext(file_id)[1:])
            fmt.Println("file length is", file_len)

            c.Data(http.StatusOK, "image/png", C.GoBytes(unsafe.Pointer(C.out_file_buffer), C.int(file_length)))
        } else {
            c.JSON(http.StatusOK, gin.H{"result": "fail",})
        }
    })

    router.Run(":8080")
}

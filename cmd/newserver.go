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

int init_fdfs(char *conf_filename)
{
	log_init();
	g_log_context.log_level = LOG_ERR;
	ignore_signal_pipe();

	return fdfs_client_init(conf_filename);
}

void destroy_fdfs()
{
    fdfs_client_destroy();
}

int fdfs_upload_file(char *filebuff, int64_t filesize, char *extname)
{
	char group_name[FDFS_GROUP_NAME_MAX_LEN + 1];
	ConnectionInfo *pTrackerServer;
	int result;
	int store_path_index;
	ConnectionInfo storageServer;

	pTrackerServer = tracker_get_connection();
	if (pTrackerServer == NULL)
	{
		return errno != 0 ? errno : ECONNREFUSED;
	}

	*group_name = '\0';
	if ((result = tracker_query_storage_store(pTrackerServer, \
	                &storageServer, group_name, &store_path_index)) != 0)
	{
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

	return result;
}


int fdfs_download_file(char *file_id, int64_t *file_size)
{
	ConnectionInfo *pTrackerServer;
	int result;
	int64_t file_offset;
	int64_t download_bytes;

	pTrackerServer = tracker_get_connection();
	if (pTrackerServer == NULL)
	{
		return errno != 0 ? errno : ECONNREFUSED;
	}

	file_offset = 0;
	download_bytes = 0;

	result = storage_do_download_file1_ex(pTrackerServer, \
                NULL, FDFS_DOWNLOAD_TO_BUFF, file_id, \
                file_offset, download_bytes, \
                &out_file_buffer, NULL, file_size);

	tracker_disconnect_server_ex(pTrackerServer, true);

	return result;
}


int fdfs_delete_file(char *file_id)
{
	ConnectionInfo *pTrackerServer;
	int result;

	pTrackerServer = tracker_get_connection();
	if (pTrackerServer == NULL)
	{
		return errno != 0 ? errno : ECONNREFUSED;
	}

	result = storage_delete_file1(pTrackerServer, NULL, file_id);

	tracker_disconnect_server_ex(pTrackerServer, true);

	return result;
}

#cgo CFLAGS: -I/usr/include/fastcommon -I/usr/include/fastdfs
#cgo LDFLAGS: -L/usr/lib64 -lpthread -lfastcommon -lfdfsclient
*/
import "C"

func main() {

    confstr := C.CString("/etc/fdfs/client.conf")
    defer C.free(unsafe.Pointer(confstr))
    result := int(C.init_fdfs(confstr))

    if result != 0 {
        fmt.Println("init fdfs error!")
        return
    }

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

        extstr := C.CString(path.Ext(filename)[1:])
        defer C.free(unsafe.Pointer(extstr))
        result := int(C.fdfs_upload_file((*C.char)(unsafe.Pointer(&buff[0])), C.int64_t(len(buff)), extstr))

        if result == 0 {
            file_id := C.GoString(&C.out_file_id[0])
            fmt.Println("file id is", file_id)
            c.JSON(http.StatusOK, gin.H{"result": "success", "file_id": file_id, "key": "i am a key",})
        } else {
            c.JSON(http.StatusOK, gin.H{"result": "fail",})
        }

    })

    router.GET("/delete", func(c *gin.Context) {
        file_id := c.Query("fileid")
        fileidstr := C.CString(file_id)
        defer C.free(unsafe.Pointer(fileidstr))

        result := int(C.fdfs_delete_file(fileidstr))

        if result == 0 {
            file_id := C.GoString(&C.out_file_id[0])
            fmt.Println("delete file:", file_id)
            c.JSON(http.StatusOK, gin.H{"result": "success",})
        } else {
            c.JSON(http.StatusOK, gin.H{"result": "fail",})
        }

    })

    router.GET("/getimage", func(c *gin.Context) {
        file_id := c.Query("fileid")
        fileidstr := C.CString(file_id)
        defer C.free(unsafe.Pointer(fileidstr))
        var file_length C.int64_t

        result := int(C.fdfs_download_file(fileidstr, &file_length))

        if result == 0 {
            defer C.free(unsafe.Pointer(C.out_file_buffer))
            file_len := int(file_length)
            c.Header("Content-Length", strconv.Itoa(file_len))
            fmt.Println("file id is", file_id)
            fmt.Println("file content type is", path.Ext(file_id)[1:])
            fmt.Println("file length is", file_len)

            c.Data(http.StatusOK, "image/" + path.Ext(file_id)[1:], C.GoBytes(unsafe.Pointer(C.out_file_buffer), C.int(file_length)))
        } else {
            c.JSON(http.StatusNotFound, gin.H{"result": "not found",})
        }
    })

    router.Run(":8080")
}

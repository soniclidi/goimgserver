package main


import (
    "fmt"
    "log"
    "net/http"
    "html/template"
    "unsafe"
    "io/ioutil"
    "path"
    "strconv"
    "crypto/md5"
    "encoding/hex"
    "math/rand"
    "time"
    "flag"
    "mime"

    "config"

    "gopkg.in/mgo.v2"
    "github.com/gin-gonic/gin"
    "gopkg.in/mgo.v2/bson"
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

type File struct {
    File_id    string
    File_name  string
    File_md5   string
    File_token string
    File_owner_id    string
    File_upload_time int
}

var configFile = flag.String("conf", "./config.json", "the path of the config.")

func main() {

    flag.Parse()

    conf, err := config.Load(*configFile)
    if err != nil {
        panic(err)
    }

    mgo, err := mgo.Dial(conf.DataBase.IP + ":" + strconv.Itoa(conf.DataBase.Port))
    if err != nil {
        panic(err)
    }
    defer mgo.Close()

    confStr := C.CString(conf.FdfsClient.ConfigFile)
    defer C.free(unsafe.Pointer(confStr))
    result := int(C.init_fdfs(confStr))

    if result != 0 {
        panic("init fdfs error!")
    }

    db := mgo.DB(conf.DataBase.DB)
    collection := db.C(conf.DataBase.Collection)

    router := gin.Default()
    html := template.Must(template.ParseFiles(conf.WebServer.Template))
    router.SetHTMLTemplate(html)

    router.GET("/upload", func(c *gin.Context) {
        c.HTML(http.StatusOK, "upload.tmpl", nil)
    })

    router.POST("/upload", func(c *gin.Context) {
        file, header, err := c.Request.FormFile("file")
        fileName := header.Filename

        buff, err := ioutil.ReadAll(file)
        if err != nil {
            log.Fatal(err)
        }

        md5Ctx := md5.New()
        md5Ctx.Write(buff)
        md5Str := hex.EncodeToString(md5Ctx.Sum(nil))
        fmt.Println("file md5:", md5Str)

        query := collection.Find(bson.M{"file_md5": md5Str})
        count, _ := query.Count()

        var result int = -1
        var fileId string
        //new file
        if count == 0 {
            extStr := C.CString(path.Ext(fileName)[1:])
            defer C.free(unsafe.Pointer(extStr))
            result = int(C.fdfs_upload_file((*C.char)(unsafe.Pointer(&buff[0])), C.int64_t(len(buff)), extStr))
            if result == 0 {
                fileId = C.GoString(&C.out_file_id[0])
                fmt.Println("new file id:", fileId)
            }
        } else {
            //get fileId from db
            existFile := File{}
            err = query.One(&existFile)
            if err == nil {
                fileId = existFile.File_id
                fmt.Println("exist file id:", existFile.File_id)
                result = 0
            }
        }


        if result == 0 {
            fileToken := string(Krand(10, KC_RAND_KIND_ALL))
            fmt.Println("file token:", fileToken)
            newFile := &File{
                File_id: fileId,
                File_name: fileName,
                File_owner_id: c.PostForm("uid"),
                File_md5: md5Str,
                File_token: fileToken,
                File_upload_time: int(time.Now().Unix()),
            }

            dberr := collection.Insert(newFile)
            if dberr != nil {
                fmt.Println(err)
                // to do: do something
                c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "insert to db error",})
            } else {
                c.JSON(http.StatusOK, gin.H{"result": "success", "file_id": fileId, "file_token": fileToken,})
            }
        } else {
            c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "upload file error",})
        }

    })

    router.GET("/delete", func(c *gin.Context) {
        fileToken := c.Query("filetoken")
        fileId := c.Query("fileid")
        fileIdStr := C.CString(fileId)
        defer C.free(unsafe.Pointer(fileIdStr))

        err := collection.Remove(bson.M{"file_token": fileToken, "file_id": fileId})

        if err == nil {
            count, _ := collection.Find(bson.M{"file_id": fileId}).Count()
            if count == 0 {
                result := int(C.fdfs_delete_file(fileIdStr))

                if result == 0 {
                    fmt.Println("delete file:", fileId)
                    c.JSON(http.StatusOK, gin.H{"result": "success",})
                } else {
                    c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "delete file error",})
                }
            } else {
                c.JSON(http.StatusOK, gin.H{"result": "success",})
            }
        } else {
            c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "remove from db error",})
        }

    })

    router.GET("/exist", func(c *gin.Context) {
        md5 := c.Query("md5")
        count, err := collection.Find(bson.M{"file_md5": md5}).Count()

        if err != nil || count == 0 {
            c.JSON(http.StatusOK, gin.H{"exist": "no",})
        } else {
            c.JSON(http.StatusOK, gin.H{"exist": "yes",})
        }

    })

    router.GET("/getimage", func(c *gin.Context) {
        fileId := c.Query("fileid")
        fileIdStr := C.CString(fileId)
        defer C.free(unsafe.Pointer(fileIdStr))
        var file_length C.int64_t

        result := int(C.fdfs_download_file(fileIdStr, &file_length))

        if result == 0 {
            defer C.free(unsafe.Pointer(C.out_file_buffer))
            fileLen := int(file_length)
            c.Header("Content-Length", strconv.Itoa(fileLen))
            fmt.Println("file id is", fileId)
            fmt.Println("file content type is", path.Ext(fileId)[1:])
            fmt.Println("file length is", fileLen)

            contentType := mime.TypeByExtension(path.Ext(fileId))

            c.Data(http.StatusOK, contentType, C.GoBytes(unsafe.Pointer(C.out_file_buffer), C.int(file_length)))
        } else {
            c.JSON(http.StatusNotFound, gin.H{"result": "not found",})
        }
    })

    router.Run(":" + strconv.Itoa(conf.WebServer.Port))
}

const (
    KC_RAND_KIND_NUM   = 0  // 纯数字
    KC_RAND_KIND_LOWER = 1  // 小写字母
    KC_RAND_KIND_UPPER = 2  // 大写字母
    KC_RAND_KIND_ALL   = 3  // 数字、大小写字母
)

func Krand(size int, kind int) []byte {
    ikind, kinds, result := kind, [][]int{[]int{10, 48}, []int{26, 97}, []int{26, 65}}, make([]byte, size)
    is_all := kind > 2 || kind < 0
    rand.Seed(time.Now().UnixNano())
    for i :=0; i < size; i++ {
        if is_all { // random ikind
            ikind = rand.Intn(3)
        }
        scope, base := kinds[ikind][0], kinds[ikind][1]
        result[i] = uint8(base+rand.Intn(scope))
    }
    return result
}
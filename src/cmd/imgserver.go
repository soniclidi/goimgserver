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
    "time"
    "flag"
    "encoding/base64"
    "strings"
    "bytes"
    "config"
    "mymime"

    "github.com/pborman/uuid"
    "gopkg.in/mgo.v2"
    "gopkg.in/gin-gonic/gin.v1"
    "gopkg.in/mgo.v2/bson"
    "github.com/disintegration/imaging"
    "gopkg.in/gin-contrib/cors.v1"
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
    File_dir_id      bson.ObjectId
    File_upload_time int
}

type Dir struct {
    Dir_id    bson.ObjectId `bson:"_id"`
    Dir_name  string
    Dir_level int
    Parent_id bson.ObjectId
    Dir_owner_id string
}

var configFile = flag.String("conf", "./config.json", "the path of the config.")
var rootDirId = "583fbc0d149f29904ec4f166"
var filesCollection *mgo.Collection
var dirsCollection  *mgo.Collection

func main() {
    flag.Parse()

    conf, err := config.Load(*configFile)
    if err != nil {
        panic(err)
    }

    err = mymime.Load(conf.WebServer.MimeTypes)
    if err != nil {
        fmt.Println("load mime types file error!")
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
    filesCollection = db.C(conf.DataBase.FilesCollection)
    dirsCollection  = db.C(conf.DataBase.DirsCollection)

    router := gin.Default()
    router.Use(cors.Default())
    html := template.Must(template.ParseFiles(conf.WebServer.Template))
    router.SetHTMLTemplate(html)

    router.GET("/upload", func(c *gin.Context) {
        c.HTML(http.StatusOK, "upload.tmpl", nil)
    })

    router.POST("/upload", func(c *gin.Context) {
        dirId := c.PostForm("dir_id")
        if len(dirId) == 0 {
            dirId = rootDirId
        }
        if !bson.IsObjectIdHex(dirId) {
            c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "invalid parameter: dir_id"})
            return
        }

        file, header, err := c.Request.FormFile("file")
        if header == nil {
            fmt.Println("=======================null header")
        }
        fileName := header.Filename


        buff, err := ioutil.ReadAll(file)
        if err != nil {
            log.Fatal(err)
        }
        ownerId := c.PostForm("owner_id")

        result, fileId, fileToken := doUpload(buff, fileName, ownerId, dirId)

        if result == 0 {
            c.JSON(http.StatusOK, gin.H{"result": "success", "file_id": fileId, "file_token": fileToken,})
        } else if result == -1 {
            c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "insert to db error",})
        } else if result == -2 {
            c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "upload file error",})
        }

    })

    router.GET("/delete", doDelete)
    router.GET("/exist", doExist)
    router.GET("/get", doGet)
    router.GET("/getimage", doGetImage)
    router.GET("/info", doInfo)
    router.GET("/mkdir", doMkDir)
    router.GET("/rmdir", doRmDir)
    router.GET("/listdir", doListDir)
    router.GET("/listrootdir", doListRootDir)

    router.Run(":" + strconv.Itoa(conf.WebServer.Port))
}

func doDelete(c *gin.Context) {
    fileToken := c.Query("file_token")
    fileId := c.Query("file_id")
    fileIdStr := C.CString(fileId)
    defer C.free(unsafe.Pointer(fileIdStr))

    err := filesCollection.Remove(bson.M{"file_token": fileToken, "file_id": fileId})

    if err == nil {
        count, _ := filesCollection.Find(bson.M{"file_id": fileId}).Count()
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
}

func doExist(c *gin.Context) {
    md5 := c.Query("file_md5")
    count, err := filesCollection.Find(bson.M{"file_md5": md5}).Count()

    if err != nil || count == 0 {
        c.JSON(http.StatusOK, gin.H{"exist": "no",})
    } else {
        c.JSON(http.StatusOK, gin.H{"exist": "yes",})
    }
}

func doGet(c *gin.Context) {
    fileId := c.Query("file_id")
    fmt.Println("get by file id: ", fileId)
    if fileId == "" {
        fmt.Println("file id is empty")
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "not found"})
        return
    }

    fileIdStr := C.CString(fileId)
    defer C.free(unsafe.Pointer(fileIdStr))
    var file_length C.int64_t

    result := int(C.fdfs_download_file(fileIdStr, &file_length))

    if result == 0 {
        defer C.free(unsafe.Pointer(C.out_file_buffer))
        originalExt := c.Query("original_ext")
        fileLen := int(file_length)
        c.Header("Content-Length", strconv.Itoa(fileLen))

        contentType := mymime.TypeByExt(path.Ext(fileId)[1:])
        if originalExt == "true" {
            fileToken := c.Query("file_token")
            existFile := File{}
            err := filesCollection.Find(bson.M{"file_token": fileToken, "file_id": fileId}).One(&existFile)
            if err == nil {
                contentType = mymime.TypeByExt(path.Ext(existFile.File_name)[1:])
            }
        }

        fmt.Println("file id is", fileId)
        fmt.Println("file content type is", contentType)
        fmt.Println("file length is", fileLen)

        c.Data(http.StatusOK, contentType, C.GoBytes(unsafe.Pointer(C.out_file_buffer), C.int(file_length)))
    } else {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "not found"})
    }
}

func doGetImage(c *gin.Context) {
    formats := map[string]imaging.Format{
        ".jpg":  imaging.JPEG,
        ".jpeg": imaging.JPEG,
        ".png":  imaging.PNG,
        ".tif":  imaging.TIFF,
        ".tiff": imaging.TIFF,
        ".bmp":  imaging.BMP,
        ".gif":  imaging.GIF,
    }

    width, err := strconv.Atoi(c.Query("width"))
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "invalid parameter: width"})
        return
    }
    height, err := strconv.Atoi(c.Query("height"))
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "invalid parameter: height"})
        return
    }
    if height <= 0 || width <= 0 {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "invalid parameter: width or height"})
        return
    }

    fileId := c.Query("file_id")
    ext := strings.ToLower(path.Ext(fileId))
    f, ok := formats[ext]
    if !ok {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "unsupported image format"})
        return
    }

    fileIdStr := C.CString(fileId)
    defer C.free(unsafe.Pointer(fileIdStr))
    var file_length C.int64_t

    result := int(C.fdfs_download_file(fileIdStr, &file_length))

    if result == 0 {
        defer C.free(unsafe.Pointer(C.out_file_buffer))

        srcBuffer := bytes.NewBuffer(C.GoBytes(unsafe.Pointer(C.out_file_buffer), C.int(file_length)))
        srcImage, err := imaging.Decode(srcBuffer)
        if err != nil {
            c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "decode image file error"})
            return
        }

        dstImage := imaging.Resize(srcImage, width, height, imaging.Lanczos)
        dstBuffer := new(bytes.Buffer)
        imaging.Encode(dstBuffer, dstImage, f)

        c.Header("Content-Length", strconv.Itoa(dstBuffer.Len()))
        contentType := mymime.TypeByExt(path.Ext(fileId)[1:])

        c.Data(http.StatusOK, contentType, dstBuffer.Bytes())
    } else {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "file not found"})
    }
}

func doInfo(c *gin.Context) {
    fileToken := c.Query("file_token")
    existFile := File{}
    err := filesCollection.Find(bson.M{"file_token": fileToken}).One(&existFile)

    if err == nil {
        c.JSON(http.StatusOK, gin.H{"result": "success", "data": existFile,})
    } else {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "get file info error",})
    }
}

func doMkDir(c *gin.Context) {
    dirName := c.Query("dir_name")
    dirLevel, err := strconv.Atoi(c.Query("dir_level"))
    if err != nil || dirLevel < 1 || dirLevel > 10 {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "invalid parameter: dir_level"})
        return
    }
    dirOwnerId := c.Query("dir_owner_id")
    parentId := c.Query("parent_id")
    if len(parentId) == 0 {
        parentId = rootDirId
    }
    if !bson.IsObjectIdHex(parentId) {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "invalid parameter: parent_id"})
        return
    }

    parentObjId := bson.ObjectIdHex(parentId)
    existDir := Dir{}
    err = dirsCollection.Find(bson.M{"dir_name": dirName, "dir_level": dirLevel,
        "dir_owner_id": dirOwnerId, "parent_id": parentObjId}).One(&existDir)

    if err == nil {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "dir already exist"})
        return
    }

    dirId := bson.NewObjectId()
    newDir := &Dir{
        Dir_id: dirId,
        Dir_name: dirName,
        Dir_level: dirLevel,
        Parent_id: parentObjId,
        Dir_owner_id: dirOwnerId,
    }

    dberr := dirsCollection.Insert(newDir)
    if dberr != nil {
        fmt.Println(dberr)
        // to do: do something

        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "insert to db error",})
    } else {
        c.JSON(http.StatusOK, gin.H{"result": "success", "dir_id": dirId,})
    }
}

func doRmDir(c *gin.Context) {
    if !bson.IsObjectIdHex(c.Query("dir_id")) {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "invalid parameter: dir_id"})
        return
    }
    dirId := bson.ObjectIdHex(c.Query("dir_id"))

    count, err := filesCollection.Find(bson.M{"file_dir_id": dirId}).Count()

    if err == nil {
        if count == 0 {
            err = dirsCollection.Remove(bson.M{"_id": dirId, "dir_owner_id": c.Query("owner_id")})
            if err == nil {
                c.JSON(http.StatusOK, gin.H{"result": "success",})
            } else {
                c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "remove from db error",})
            }
        } else {
            c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "can not remove a directory that contains files",})
        }
    } else {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "db operaion error",})
    }
}

func doListDir(c *gin.Context) {
    if !bson.IsObjectIdHex(c.Query("dir_id")) {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "invalid parameter: dir_id"})
        return
    }
    dirId := bson.ObjectIdHex(c.Query("dir_id"))

    existFiles := []File{}
    ferr := filesCollection.Find(bson.M{"file_dir_id": dirId}).All(&existFiles)

    existDirs := []Dir{}
    derr := dirsCollection.Find(bson.M{"parent_id": dirId}).All(&existDirs)

    if ferr == nil && derr == nil {
        c.JSON(http.StatusOK, gin.H{"result": "success", "data": gin.H{"dirs": existDirs, "files": existFiles}})
    } else {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "get info from db error",})
    }
}

func doListRootDir(c *gin.Context) {
    owner_id := c.Query("owner_id")
    dirId := bson.ObjectIdHex(rootDirId)

    existFiles := []File{}
    ferr := filesCollection.Find(bson.M{"file_dir_id": dirId, "file_owner_id": owner_id}).All(&existFiles)

    existDirs := []Dir{}
    derr := dirsCollection.Find(bson.M{"parent_id": dirId, "dir_owner_id": owner_id}).All(&existDirs)

    if ferr == nil && derr == nil {
        c.JSON(http.StatusOK, gin.H{"result": "success", "data": gin.H{"dirs": existDirs, "files": existFiles}})
    } else {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "get info from db error",})
    }
}

func doUpload(fileBuff []byte, fileName string, ownerId string, dirId string) (int, string, string) {
    md5Ctx := md5.New()
    md5Ctx.Write(fileBuff)
    md5Str := hex.EncodeToString(md5Ctx.Sum(nil))
    fmt.Println("file md5:", md5Str)

    query := filesCollection.Find(bson.M{"file_md5": md5Str})
    count, _ := query.Count()

    var result int = -1
    var fileId string
    //new file
    if count == 0 {
        extStr := C.CString(path.Ext(fileName)[1:])
        defer C.free(unsafe.Pointer(extStr))
        result = int(C.fdfs_upload_file((*C.char)(unsafe.Pointer(&fileBuff[0])), C.int64_t(len(fileBuff)), extStr))
        if result == 0 {
            fileId = C.GoString(&C.out_file_id[0])
            fmt.Println("new file id:", fileId)
        }
    } else {
        //get fileId from db
        existFile := File{}
        err := query.One(&existFile)
        if err == nil {
            fileId = existFile.File_id
            fmt.Println("exist file id:", existFile.File_id)
            result = 0
        }
    }

    if result == 0 {
        fileToken := genToken()
        fmt.Println("file token:", fileToken)
        newFile := &File{
            File_id: fileId,
            File_name: fileName,
            File_owner_id: ownerId,
            File_dir_id: bson.ObjectIdHex(dirId),
            File_md5: md5Str,
            File_token: fileToken,
            File_upload_time: int(time.Now().Unix()),
        }

        dberr := filesCollection.Insert(newFile)
        if dberr != nil {
            fmt.Println(dberr)
            // to do: do something
            return -1, "", ""
            //c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "insert to db error",})
        } else {
            return 0, fileId, fileToken
            //c.JSON(http.StatusOK, gin.H{"result": "success", "file_id": fileId, "file_token": fileToken,})
        }
    } else {
        return -2, "", ""
        //c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "upload file error",})
    }

}

func genToken() string {
    id := uuid.NewRandom()
    return strings.TrimRight(base64.URLEncoding.EncodeToString([]byte(id)), "=")
}

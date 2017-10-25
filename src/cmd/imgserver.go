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
    "github.com/gin-gonic/gin"
    "gopkg.in/mgo.v2/bson"
    "github.com/disintegration/imaging"
    "github.com/gin-contrib/cors"
    "sync"
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


//char out_file_id[128];
//char *out_file_buffer;

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

int fdfs_upload_file(char *file_buff, int64_t file_size, char *ext_name, char *out_file_id)
{
	char group_name[FDFS_GROUP_NAME_MAX_LEN + 1];
	ConnectionInfo pTrackerServer;
	ConnectionInfo *conn;
	int result;
	int store_path_index;
	ConnectionInfo storageServer;

	//conn = tracker_get_connection();
	conn = tracker_get_connection_r(&pTrackerServer, &result);
	if (conn == NULL)
	{
		return errno != 0 ? errno : ECONNREFUSED;
	}

	*group_name = '\0';
	if ((result = tracker_query_storage_store(conn, \
	                &storageServer, group_name, &store_path_index)) != 0)
	{
		return result;
	}

	//result = storage_upload_by_filename1(conn, \
	//		&storageServer, store_path_index, \
	//		local_filename, NULL, \
	//		NULL, 0, group_name, out_file_id);
	result = storage_upload_by_filebuff1(conn, \
			&storageServer, store_path_index, \
			file_buff, file_size, ext_name, \
			NULL, 0, group_name, out_file_id);

	tracker_disconnect_server_ex(conn, true);

	return result;
}


int fdfs_download_file(char *file_id, int64_t *file_size, char **out_file_buffer)
{
	ConnectionInfo pTrackerServer;
	ConnectionInfo *conn;
	int result;
	int64_t file_offset;
	int64_t download_bytes;

	//conn = tracker_get_connection();
	conn = tracker_get_connection_r(&pTrackerServer, &result);
	if (conn == NULL)
	{
		return errno != 0 ? errno : ECONNREFUSED;
	}

	file_offset = 0;
	download_bytes = 0;

	result = storage_do_download_file1_ex(conn, \
                NULL, FDFS_DOWNLOAD_TO_BUFF, file_id, \
                file_offset, download_bytes, \
                out_file_buffer, NULL, file_size);

	tracker_disconnect_server_ex(conn, true);

	return result;
}


int fdfs_delete_file(char *file_id)
{
	ConnectionInfo pTrackerServer;
	ConnectionInfo *conn;
	int result;

	//conn = tracker_get_connection();
	conn = tracker_get_connection_r(&pTrackerServer, &result);
	if (conn == NULL)
	{
		return errno != 0 ? errno : ECONNREFUSED;
	}

	result = storage_delete_file1(conn, NULL, file_id);

	tracker_disconnect_server_ex(conn, true);

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
var conf *config.Config

var lock sync.Mutex

func main() {
    flag.Parse()

    var err error
    conf, err = config.Load(*configFile)
    if err != nil {
        panic(err)
    }

    defer C.destroy_fdfs()
    if initFdfs() == false {
        panic("init fdfs error")
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


    db := mgo.DB(conf.DataBase.DB)
    filesCollection = db.C(conf.DataBase.FilesCollection)
    dirsCollection  = db.C(conf.DataBase.DirsCollection)

    router := gin.Default()

    corsConfig := cors.DefaultConfig()
    corsConfig.AllowAllOrigins = true
    corsConfig.AddAllowHeaders("authorization")

    router.Use(cors.New(corsConfig))
    html := template.Must(template.ParseFiles(conf.WebServer.Template))
    router.SetHTMLTemplate(html)

    router.GET("/upload", func(c *gin.Context) {
        c.HTML(http.StatusOK, "upload.tmpl", nil)
    })

    router.POST("/upload", func(c *gin.Context) {
        //defer C.destroy_fdfs()
        //if initFdfs() == false {
        //    c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "init fdfs error",})
        //    return
        //}

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


func initFdfs() bool {
    confStr := C.CString(conf.FdfsClient.ConfigFile)
    defer C.free(unsafe.Pointer(confStr))
    result := int(C.init_fdfs(confStr))
    if result != 0 {
        return false
    }
    return true
}

func doDelete(c *gin.Context) {
    //defer C.destroy_fdfs()
    //if initFdfs() == false {
    //    c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "init fdfs error",})
    //    return
    //}
    //defer lock.Unlock()
    //lock.Lock()

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
    //defer C.destroy_fdfs()
    //if initFdfs() == false {
    //    c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "init fdfs error",})
    //    return
    //}
    //defer lock.Unlock()
    //lock.Lock()

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

    var out_file_buffer *C.char

    result := int(C.fdfs_download_file(fileIdStr, &file_length, &out_file_buffer))

    if result == 0 {
        defer C.free(unsafe.Pointer(out_file_buffer))
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

        c.Data(http.StatusOK, contentType, C.GoBytes(unsafe.Pointer(out_file_buffer), C.int(file_length)))
    } else {
        c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "not found"})
    }
}

func doGetImage(c *gin.Context) {
    //defer C.destroy_fdfs()
    //if initFdfs() == false {
    //    c.JSON(http.StatusOK, gin.H{"result": "fail", "desc": "init fdfs error",})
    //    return
    //}
    //defer lock.Unlock()
    //lock.Lock()

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

    var out_file_buffer *C.char

    result := int(C.fdfs_download_file(fileIdStr, &file_length, &out_file_buffer))

    if result == 0 {
        defer C.free(unsafe.Pointer(out_file_buffer))

        srcBuffer := bytes.NewBuffer(C.GoBytes(unsafe.Pointer(out_file_buffer), C.int(file_length)))
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
    //defer lock.Unlock()
    //lock.Lock()

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
        var fileIdBuff [256]byte
        result = int(C.fdfs_upload_file((*C.char)(unsafe.Pointer(&fileBuff[0])), C.int64_t(len(fileBuff)), extStr,
            (*C.char)(unsafe.Pointer(&fileIdBuff[0]))))
        if result == 0 {
            fileId = byte2String(fileIdBuff[:])
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

func byte2String(p []byte) string {
    for i := 0; i < len(p); i++ {
        if p[i] == 0 {
            return string(p[0:i])
        }
    }
    return string(p)
}

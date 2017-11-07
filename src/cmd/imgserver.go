package main


import (
    "fmt"
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
    "errors"

    "config"
    "mymime"
    "github.com/pborman/uuid"
    "gopkg.in/mgo.v2"
    "github.com/gin-gonic/gin"
    "gopkg.in/mgo.v2/bson"
    "github.com/disintegration/imaging"
    "github.com/gin-contrib/cors"
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

int fdfs_upload_image_file(char *file_buff, int64_t file_size, char *thumb_buff, int64_t thumb_size, \
        char *prefix_name, char *ext_name, char *out_file_id, char *thumb_file_id)
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
	if ((result = tracker_query_storage_store(conn, &storageServer, group_name, &store_path_index)) != 0)
	{
		return result;
	}

	result = storage_upload_by_filebuff1(conn, \
			&storageServer, store_path_index, \
			file_buff, file_size, ext_name, \
			NULL, 0, group_name, out_file_id);

	if (result != 0)
	{
	    return result;
	}

	result = storage_upload_slave_by_filebuff1(conn, &storageServer, thumb_buff, thumb_size, out_file_id, \
	        prefix_name, ext_name, NULL, 0, thumb_file_id);

	if (result != 0)
	{
	    storage_delete_file1(conn, NULL, out_file_id);
	    return result;
	}

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

var thumbFileWidth  = 100
var thumbFileHeight = 80
var thumbFilePrefix = "_thumb_100x80"

var imageFormats = map[string]imaging.Format{
".jpg":  imaging.JPEG,
".jpeg": imaging.JPEG,
".png":  imaging.PNG,
".tif":  imaging.TIFF,
".tiff": imaging.TIFF,
".bmp":  imaging.BMP,
".gif":  imaging.GIF,
}

var configFile = flag.String("conf", "./config.json", "the path of the config.")
var rootDirId = "583fbc0d149f29904ec4f166"
var filesCollection *mgo.Collection
var dirsCollection  *mgo.Collection
var conf *config.Config


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


    //router.POST("/upload_image", doUploadImage)
    router.POST("/upload", doUpload)

    router.GET("/delete", doDelete)
    router.GET("/exist", doExist)
    router.GET("/get", doGet)
    router.GET("/get_image", doGetImage)
    router.GET("/info", doInfo)
    router.GET("/mk_dir", doMkDir)
    router.GET("/rm_dir", doRmDir)
    router.GET("/list_dir", doListDir)
    router.GET("/list_root_dir", doListRootDir)

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

func doUpload(c *gin.Context) {
    dirId := c.PostForm("dir_id")
    if len(dirId) == 0 {
        dirId = rootDirId
    }
    if !bson.IsObjectIdHex(dirId) {
        errorResponse(c, "invalid parameter: dir_id")
        return
    }

    file, header, err := c.Request.FormFile("file")
    if header == nil {
        errorResponse(c, "null http header")
        return
    }
    fileName := header.Filename

    buff, err := ioutil.ReadAll(file)
    if err != nil {
        fmt.Println(err)
        errorResponse(c, "read file error")
        return
    }

    ownerId := c.PostForm("owner_id")
    genThumb := false
    if c.PostForm("thumb") == "true" {
        genThumb = true
    }

    err, fileId, fileToken := uploadFile(buff, fileName, ownerId, dirId, genThumb)
    if err == nil {
        successResponse(c, gin.H{"file_id": fileId, "file_token": fileToken,})
    } else {
        fmt.Println(err.Error())
        errorResponse(c, err.Error())
    }

    //result, fileId, fileToken := uploadFile(buff, fileName, ownerId, dirId)
    //
    //if result == 0 {
    //    successResponse(c, gin.H{"file_id": fileId, "file_token": fileToken,})
    //} else if result == -1 {
    //    errorResponse(c, "insert to db error")
    //} else if result == -2 {
    //    errorResponse(c, "upload file error")
    //}
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
                successResponse(c, nil)
            } else {
                errorResponse(c, "delete file error")
            }
        } else {
            successResponse(c, nil)
        }
    } else {
        errorResponse(c, "remove from db error")
    }
}

func doExist(c *gin.Context) {
    md5 := c.Query("file_md5")
    count, err := filesCollection.Find(bson.M{"file_md5": md5}).Count()

    exist := "true"
    if err != nil || count == 0 {
        exist = "false"
    }

    successResponse(c, gin.H{"exist": exist})
}

func doGet(c *gin.Context) {
    fileId := c.Query("file_id")
    fmt.Println("get by file id: ", fileId)
    if fileId == "" {
        errorResponse(c, "file not found")
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
        errorResponse(c, "file not found")
    }
}

func doGetImage(c *gin.Context) {
    width, err := strconv.Atoi(c.Query("width"))
    if err != nil {
        errorResponse(c, "invalid parameter: width")
        return
    }
    height, err := strconv.Atoi(c.Query("height"))
    if err != nil {
        errorResponse(c, "invalid parameter: height")
        return
    }
    if height <= 0 || width <= 0 {
        errorResponse(c, "invalid parameter: width or height")
        return
    }

    fileId := c.Query("file_id")
    ext := strings.ToLower(path.Ext(fileId))
    f, ok := imageFormats[ext]
    if !ok {
        errorResponse(c, "unsupported image format")
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
            errorResponse(c, "decode image file error")
            return
        }

        dstImage := imaging.Resize(srcImage, width, height, imaging.Lanczos)
        dstBuffer := new(bytes.Buffer)
        imaging.Encode(dstBuffer, dstImage, f)

        c.Header("Content-Length", strconv.Itoa(dstBuffer.Len()))
        contentType := mymime.TypeByExt(path.Ext(fileId)[1:])

        c.Data(http.StatusOK, contentType, dstBuffer.Bytes())
    } else {
        errorResponse(c, "file not found")
    }
}

func doInfo(c *gin.Context) {
    fileToken := c.Query("file_token")
    existFile := File{}
    err := filesCollection.Find(bson.M{"file_token": fileToken}).One(&existFile)

    if err == nil {
        successResponse(c, existFile)
    } else {
        errorResponse(c, "get file info error")
    }
}

func doMkDir(c *gin.Context) {
    dirName := c.Query("dir_name")
    dirLevel, err := strconv.Atoi(c.Query("dir_level"))
    if err != nil || dirLevel < 1 || dirLevel > 10 {
        errorResponse(c, "invalid parameter: dir_level")
        return
    }
    dirOwnerId := c.Query("dir_owner_id")
    parentId := c.Query("parent_id")
    if len(parentId) == 0 {
        parentId = rootDirId
    }
    if !bson.IsObjectIdHex(parentId) {
        errorResponse(c, "invalid parameter: parent_id")
        return
    }

    parentObjId := bson.ObjectIdHex(parentId)
    existDir := Dir{}
    err = dirsCollection.Find(bson.M{"dir_name": dirName, "dir_level": dirLevel,
        "dir_owner_id": dirOwnerId, "parent_id": parentObjId}).One(&existDir)

    if err == nil {
        errorResponse(c, "dir already exist")
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

        errorResponse(c, "insert to db error")
    } else {
        successResponse(c, gin.H{"dir_id": dirId,})
    }
}

func doRmDir(c *gin.Context) {
    if !bson.IsObjectIdHex(c.Query("dir_id")) {
        errorResponse(c, "invalid parameter: dir_id")
        return
    }
    dirId := bson.ObjectIdHex(c.Query("dir_id"))

    count, err := filesCollection.Find(bson.M{"file_dir_id": dirId}).Count()
    if err == nil {
        if count == 0 {
            err = dirsCollection.Remove(bson.M{"_id": dirId, "dir_owner_id": c.Query("owner_id")})
            if err == nil {
                successResponse(c, nil)
            } else {
                errorResponse(c, "remove from db error")
            }
        } else {
            errorResponse(c, "can not remove a directory that contains files")
        }
    } else {
        errorResponse(c, "db operaion error")
    }
}

func doListDir(c *gin.Context) {
    if !bson.IsObjectIdHex(c.Query("dir_id")) {
        errorResponse(c, "invalid parameter: dir_id")
        return
    }
    dirId := bson.ObjectIdHex(c.Query("dir_id"))

    existFiles := []File{}
    ferr := filesCollection.Find(bson.M{"file_dir_id": dirId}).All(&existFiles)

    existDirs := []Dir{}
    derr := dirsCollection.Find(bson.M{"parent_id": dirId}).All(&existDirs)

    if ferr == nil && derr == nil {
        successResponse(c, gin.H{"dirs": existDirs, "files": existFiles})
    } else {
        errorResponse(c, "get info from db error")
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
        successResponse(c, gin.H{"dirs": existDirs, "files": existFiles})
    } else {
        errorResponse(c, "get info from db error")
    }
}

func uploadFile(fileBuff []byte, fileName string, ownerId string, dirId string, genThumb bool) (error, string, string) {
    //defer lock.Unlock()
    //lock.Lock()

    md5Ctx := md5.New()
    md5Ctx.Write(fileBuff)
    md5Str := hex.EncodeToString(md5Ctx.Sum(nil))
    fmt.Println("file md5:", md5Str)

    query := filesCollection.Find(bson.M{"file_md5": md5Str})
    count, _ := query.Count()

    var result int = 1
    var fileId string
    var thumbFileId string
    //new file
    if count == 0 {
        extStr := C.CString(path.Ext(fileName)[1:])
        defer C.free(unsafe.Pointer(extStr))

        var fileIdBuff [256]byte
        var thumbFileIdBuff [256]byte
        if genThumb {
            prefixStr := C.CString(thumbFilePrefix)
            defer C.free(unsafe.Pointer(prefixStr))

            ext := strings.ToLower(path.Ext(fileName))
            f, ok := imageFormats[ext]
            if !ok {
                return errors.New("unsupported image format"), "", ""
            }

            srcBuffer := bytes.NewBuffer(fileBuff)
            srcImage, err := imaging.Decode(srcBuffer)
            if err != nil {
                return errors.New("decode image file error"), "", ""
            }
            dstImage := imaging.Resize(srcImage, thumbFileWidth, thumbFileHeight, imaging.Lanczos)
            thumbBuffer := new(bytes.Buffer)
            imaging.Encode(thumbBuffer, dstImage, f)

            result = int (C.fdfs_upload_image_file((*C.char)(unsafe.Pointer(&fileBuff[0])), C.int64_t(len(fileBuff)),
                (*C.char)(unsafe.Pointer(&(thumbBuffer.Bytes()[0]))), C.int64_t(thumbBuffer.Len()),
                prefixStr, extStr, (*C.char)(unsafe.Pointer(&fileIdBuff[0])),
                (*C.char)(unsafe.Pointer(&thumbFileIdBuff[0]))))
        } else {
            result = int(C.fdfs_upload_file((*C.char)(unsafe.Pointer(&fileBuff[0])), C.int64_t(len(fileBuff)), extStr,
                (*C.char)(unsafe.Pointer(&fileIdBuff[0]))))
        }

        if result == 0 {
            fileId = byte2String(fileIdBuff[:])
            fmt.Println("new file id:", fileId)
            thumbFileId = byte2String(thumbFileIdBuff[:])
            fmt.Println("new thumb file id:", thumbFileId)
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
            return errors.New("insert to db error"), "", ""
        } else {
            return nil, fileId, fileToken
        }
    } else {
        return errors.New("upload file error"), "", ""
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

func successResponse(c *gin.Context, data interface{}) {
    c.JSON(http.StatusOK, gin.H{"result": "success", "data": data,})
}

func errorResponse(c *gin.Context, desc string) {
    c.JSON(http.StatusOK, gin.H{"result": "error", "desc": desc,})
}

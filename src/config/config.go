package config

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "errors"
)

type DataBase struct {
    IP         string `json:"ip"`
    Port       int    `json:"port"`
    DB         string `json:"db"`
    Collection string `json:"collection"`
}

type WebServer struct {
    Port int `json:"port"`
    Template  string `json:"template"`
    MimeTypes string `json:"mimetypes"`
}

type FdfsClient struct {
    ConfigFile string `json:"configfile"`
}

type Config struct {
    FdfsClient `json:"fdfsclient"`
    DataBase  `json:"database"`
    WebServer `json:"webserver"`
}

func (c *Config) init(path string) error {
    var err error
    if err = c.parse(path); err != nil {
        return fmt.Errorf("Can't load config from: %s with error: %v.", path, err)
    }

    return c.check()
}

func (c *Config) parse(path string) error {
    if path == "" {
        return errors.New("Config path is empty!")
    }
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return err
    }
    err = json.Unmarshal(data, c)
    fmt.Println(c)
    return err
}

func (c *Config) check() error {
    return nil
}

func Load(path string) (*Config, error) {
    c := &Config{}
    err := c.init(path)

    if err != nil {
        return nil, err
    }
    return c, nil
}

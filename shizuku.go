package main

import (
	"github.com/gin-gonic/gin";
	"github.com/gin-gonic/gin/binding";
	"html/template"
	"log"
	"fmt"
    "gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
    "os"
    "io"
    "net/http"
    "strings"
    "time"
    "encoding/json"
)

type Nanika struct {
	Name string
	FileName string
	HitCount int
	Slug string
	Password string //while we have the option to password files... security is not a focus.
	Active bool
}

// Binding from form values
type NaniForm struct {
    Name     string `form:"name" binding:"optional"`
    FileName string `form:"filename" binding:"optional"`
    Slug 	 string `form:"slug" binding:"optional"`
    Password string `form:"password" binding:"optional"`
    Active   bool   `form:"active" binding:"optional"`
}

var (
    templateDelims = []string{"{{%", "%}}"}
	IsDrop = true
    //templates *template.Template
)

var Contains = func(list []string, elem string) bool { 
        for _, t := range list { if t == elem { return true } } 
        return false 
} 

func main() {
    r := gin.Default()
    html := template.Must(template.New("").Delims(templateDelims[0], templateDelims[1]).ParseFiles("./templates/index.tmpl","./templates/admin.tmpl"))
    r.SetHTMLTemplate(html)
    r.Static("/assets","./public")

    //setting up mongodb
    session, err := mgo.Dial("localhost")
    if (err  != nil){
    	fmt.Println("Oh gosh!")
    	panic(err)
    }
    defer session.Close()
    session.SetMode(mgo.Monotonic, true)
 
	// Drop Database if true
	if IsDrop {
		err = session.DB("shizuku").DropDatabase()
		if err != nil {
			panic(err)
		}
	}

    db := session.DB("shizuku").C("nanika")
   	index := mgo.Index{
		Key:        []string{"name", "slug"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err = db.EnsureIndex(index)
	if err != nil {
		panic(err)
	}
	
    err = db.Insert(&Nanika{Name: "SomeName", FileName: "some/file/name",HitCount: 0, Slug: "Slugtest", Password: "password", Active: true},
    	&Nanika{Name: "SomeOtherName", FileName: "some/other/file/name",HitCount: 200, Slug: "Slugtest2", Password: "password2", Active: false},
    	&Nanika{Name: "SomeOther3Name", FileName: "some/other2/file/name",HitCount: 200, Slug: "Slugtest3", Password: "", Active: true})
	if err != nil {
		panic(err)
	}
	
	/*
    result := []Nanika{}
    err = db.Find(nil).All(&result)
    if err != nil {
    	fmt.Println("Oh gosh! Didn't find it")
        log.Fatal(err)
    }

    //fmt.Println("Name: ", result.Name)
    fmt.Println("what we pulled from db: ", result)
	*/

    r.GET("/", func(c *gin.Context) {
        obj := gin.H{"title": "Apa!"}
        c.HTML(200, "index.tmpl", obj)
    })

	r.GET("/f/:slug", func(c *gin.Context) {
        obj := gin.H{"title": c.Params.ByName("slug")}
        ret := Nanika{}
        fmt.Println("slug is: "+ c.Params.ByName("slug"))
        err := db.Find(bson.M{"slug": c.Params.ByName("slug")}).One(&ret)
        if(err != nil){
        	fmt.Println("err finding slug")
        	c.HTML(500, "index.tmpl", obj)
        	return
        }

        colQuerier := bson.M{"slug": c.Params.ByName("slug")}
        change := bson.M{"$set": bson.M{"hitcount": ret.HitCount+1}}
        err = db.Update(colQuerier, change)
        if err != nil {
            panic(err)
        }
        http.ServeFile(c.Writer, c.Request, "./uploaded/"+ret.FileName)

        //c.HTML(200, "index.tmpl", obj)
    })
    // Group using gin.BasicAuth() middleware
    // gin.Accounts is a shortcut for map[string]string
    authorized := r.Group("/admin", gin.BasicAuth(gin.Accounts{
        "apa":    "opa", 																											//TODO make this run from env var
    }))
    

    //Page we upload from
    authorized.GET("/", func(c *gin.Context){
    	//user := c.MustGet(gin.AuthUserKey).(string)
    	obj := gin.H{"title": "Shizuku Administration", "content": "testing content. This is what content will look like I guess?"}
        c.HTML(200, "admin.tmpl", obj)
    })
    
    authorized.GET("/files", func(c *gin.Context){
    	results := []Nanika{}
    	err := db.Find(nil).All(&results)
    	if( err != nil){
    		fmt.Println("We had an issue with getting all the files")
    		log.Fatal(err)
    	}
    	c.JSON(200, results)
    })

    authorized.PUT("/file", func(c *gin.Context){
        defer c.Request.Body.Close()
        var v NaniForm
        if err := json.NewDecoder(c.Request.Body).Decode(&v); err != nil {
          panic(err)
        }
        fmt.Println(v)


        colQuerier := bson.M{"filename": v.FileName}
        change := bson.M{"$set": bson.M{"slug": v.Slug, "name": v.Name, "active": v.Active, "password": v.Password}}
        err = db.Update(colQuerier, change)
        if err != nil {
            panic(err)
        }

    	c.JSON(200, gin.H{"ok": "Updated"})
    })
	authorized.DELETE("/file", func(c *gin.Context){
    	//TODO find file
    	//delete file
    	//remove record from db
    	c.JSON(200, gin.H{"ok": "Deleted"})
    })

    //where the file goes~
    authorized.POST("/upload", func(c *gin.Context){
    	file, header, err := c.Request.FormFile("file")
    	if( err != nil){
    		panic(err)
    	}
    	defer file.Close()
    	
    	buff := make([]byte, 512) // see http://golang.org/pkg/net/http/#DetectContentType
        _, err = file.Read(buff)

        if err != nil {
                 fmt.Println(err)
                 os.Exit(1)
        }
        file.Seek(0,0) //because Read offsets our copy

        filetype := http.DetectContentType(buff)

        acceptableTypes := []string{"image/jpeg",
        	"image/jpg",
        	"image/gif",
        	"image/png",
        	"image/bmp",
        	"image/gif",
        	"audio/x-wav",
        	"audio/mpeg",
        	"audio/mid",
        	"application/pdf",
        	"application/zip",
        	"application/x-javascript",
        	"text/plain"}
        if(!Contains(acceptableTypes, filetype)){
        	c.JSON(200, gin.H{"ok":"Bad filetype"})
        	return
        }

		sfn := strings.Split(header.Filename,".")
		t := time.Now()
        internFileName := sfn[0] + "-" + fmt.Sprintf("%d",t.Year()) + "-" + fmt.Sprintf("%d",t.Month()) +"-"+ fmt.Sprintf("%d",t.Day()) + "." + sfn[len(sfn)-1]
        sfn = strings.Split(internFileName,".") //because I want to reuse for slugname
        //ensure file doesn't exist
		if _, err := os.Stat("./uploaded/"+internFileName); err == nil {
			fmt.Printf("file exists; processing...")
			c.JSON(500, gin.H{"ok":"file already exists"})
			return
		}
		//create a temp file
    	outFile, err := os.Create("./uploaded/"+internFileName)
    	if( err != nil){
    		fmt.Println("Unable to create a file, check privilages")
    		c.JSON(500, gin.H{"ok":"nope, couldn't make it"})
    		return
    	}

    	defer outFile.Close()
    	//copy the data into temp file
    	_, err = io.Copy(outFile, file)
		if( err != nil){
    		fmt.Println(err)
    		c.JSON(500, gin.H{"ok":"couldn't copy"})
    		return
    	}

    	//other form vals
    	var form NaniForm
        c.BindWith(&form, binding.Form)

        //create database entry
    	nani := Nanika{
    		Name: form.Name,
    		FileName: internFileName,
    		HitCount: 0,
    		Slug: sfn[0], //should just be filename without ext
    		Password: form.Password,
    		Active: form.Active,
    	}
    	err = db.Insert(&nani)
		if err != nil {
			fmt.Println(err)
			c.JSON(500, gin.H{"ok":"error saving to db"})
			return
		}

    	c.JSON(200, gin.H{"ok":"seemed to make it through " + nani.FileName})
    })
	
    // Listen and server on 0.0.0.0:8080
    r.Run(":8080")
}
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/protocol/page"
	"github.com/mafredri/cdp/rpcc"
)

type Tweet struct {
	URL        string `json:"url"`
	Screenname string `json:"user__screen_name"`
}

func main() {
	// Configure gin webhook
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.POST("/tss", tss)

	r.Run(":32500") // listen and serve on 127.0.0.1:32500
}
func tss(c *gin.Context) {
	// Read Request Body
	reqBody, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.String(400, "Cannot read request body. Error: %v", err)
		//log.Fatalf("Cannot read request body. Error: %v", err)
		return
	}
	// Unmarshall JSON
	var tweetData Tweet
	err = json.Unmarshal(reqBody, &tweetData)
	if err != nil {
		c.String(400, "Cannot unmarshal JSON. Error: %v", err)
		//log.Fatalf("Cannot unmarshal JSON. Error: %v", err)
		return
	}
	screenshotSuccess := screenshot(tweetData.Screenname, tweetData.URL)

	if screenshotSuccess {
		c.Status(200)
	}
	if !screenshotSuccess {
		c.String(400, "Screenshot unsuccessful.")
	}
}
func screenshot(screenname string, tweetURL string) bool {
	// Setup Chrome
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the DevTools json API to get the current page.
	devt := devtool.New("http://127.0.0.1:9222")
	pageTarget, err := devt.Get(ctx, devtool.Page)
	if err != nil {
		pageTarget, err = devt.Create(ctx)
		if err != nil {
			panic(err)
		}
	}

	// Connect to Chrome Debugging Protocol target.
	conn, err := rpcc.DialContext(ctx, pageTarget.WebSocketDebuggerURL)
	if err != nil {
		panic(err)
	}
	defer conn.Close() // Must be closed when we are done.

	// Create a new CDP Client that uses conn.
	c := cdp.NewClient(conn)

	// Enable events on the Page domain.
	if err = c.Page.Enable(ctx); err != nil {
		panic(err)
	}

	// New DOMContentEventFired client will receive and buffer
	// ContentEventFired events from now on.
	domContentEventFired, err := c.Page.DOMContentEventFired(ctx)
	if err != nil {
		panic(err)
	}
	defer domContentEventFired.Close()

	// Create the Navigate arguments with the optional Referrer field set.
	navArgs := page.NewNavigateArgs(tweetURL).SetReferrer("https://duckduckgo.com")
	nav, err := c.Page.Navigate(ctx, navArgs)
	if err != nil {
		panic(err)
	}

	// Block until a DOM ContentEventFired event is triggered.
	if _, err = domContentEventFired.Recv(); err != nil {
		panic(err)
	}

	fmt.Printf("Page loaded with frame ID: %s\n", nav.FrameID)

	// Capture a screenshot of the current page.
	screenshotName := time.Now().Format("2006-01-02 15.04.05") + "_" + screenname + "_screenshot.jpg"
	screenshotArgs := page.NewCaptureScreenshotArgs().
		SetFormat("jpeg").
		SetQuality(80)
	screenshot, err := c.Page.CaptureScreenshot(ctx, screenshotArgs)
	if err != nil {
		panic(err)
	}
	if err = ioutil.WriteFile(screenshotName, screenshot.Data, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Saved screenshot: %s\n", screenshotName)

	if err != nil {
		return false
	}
	return true
}

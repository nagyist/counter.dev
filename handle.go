package main

import (
	"./models"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
)

//{
//
//sites: {
//    {
//      "example.com":
//        count: 56
//        log: ["da"]
//        visits: {
//          day: {}
//          month: {}
//          year: {}
//          all: {}
//        }
//    }
//}
//user: {
//    id: "adsf"
//    token: "asdf",
//    prefs: {
//      range: "da",
//      site: "du",
//    }
//}
//
//}

type UserDump struct {
	Id    string            `json:"id"`
	Token string            `json:"token"`
	Prefs map[string]string `json:"prefs"`
}

type SitesDumpVal struct {
	Count  int                `json:"sites"`
	Logs   models.LogData     `json:"logs"`
	Visits models.TimedVisits `json:"visits"`
}

type SitesDump map[string]SitesDumpVal

type Dump struct {
	Sites SitesDump `json:"sites"`
	User  UserDump  `json:"user"`
}

func (ctx Ctx) handleLogin() {
	userId := ctx.r.FormValue("user")
	passwordInput := ctx.r.FormValue("password")
	if userId == "" || passwordInput == "" {
		ctx.ReturnBadRequest("Missing Input")
	}

	user := ctx.app.OpenUser(userId)
	defer user.Close()

	passwordOk, err := user.VerifyPassword(passwordInput)
	ctx.CatchError(err)
	tokenOk, err := user.VerifyToken(passwordInput)
	ctx.CatchError(err)

	if passwordOk || tokenOk {
		if passwordOk {
			user.TouchAccess()
		}
		ctx.SetSessionUser(userId)
		ctx.ReturnUser()

	} else {
		ctx.ReturnBadRequest("Wrong username or password")
	}
}

func (ctx Ctx) handleLogout() {
	ctx.Logout()
	http.Redirect(ctx.w, ctx.r, "/app", http.StatusTemporaryRedirect)
}

func (ctx Ctx) handleRegister() {
	userId := ctx.r.FormValue("user")
	password := ctx.r.FormValue("password")
	if userId == "" || password == "" {
		ctx.ReturnBadRequest("Missing Input")
	}

	user := ctx.app.OpenUser(userId)
	defer user.Close()

	err := user.Create(password)
	switch err.(type) {
	case nil:
		ctx.SetSessionUser(userId)
		ctx.ReturnUser()

	case *models.ErrCreate:
		ctx.ReturnBadRequest(err.Error())

	default:
		ctx.ReturnInternalError(err)
	}
}

func (ctx Ctx) handleSetPrefRange() {
	user := ctx.ForceUser()
	defer user.Close()
	err := user.SetPref("range", ctx.r.URL.RawQuery)
	ctx.CatchError(err)

}

func (ctx Ctx) handleSetPrefSite() {
	user := ctx.ForceUser()
	defer user.Close()
	err := user.SetPref("site", ctx.r.URL.RawQuery)
	ctx.CatchError(err)

}

type PingDataResp struct {
	Visits    models.TimedVisits `json:"visits"`
	Logs      models.LogData     `json:"logs"`
	SiteLinks map[string]int     `json:"site_links"`
}

func (ctx Ctx) handlePing() {
	siteId := ctx.r.URL.RawQuery
	if siteId == "" {
		ctx.ReturnBadRequest("no siteId given as raw query param")
	}
	user := ctx.ForceUser()
	defer user.Close()
	visits := user.NewSite(siteId)

	// if parameter wait is set:
	//err := visits.WaitForSignal()
	//ctx.CatchError(err)

	timedVisits, err := visits.GetVisits(ctx.ParseUTCOffset("utcoffset"))
	ctx.CatchError(err)
	logs, err := visits.GetLogs()
	ctx.CatchError(err)
	siteLinks, err := user.GetSiteLinks()
	ctx.CatchError(err)
	resp := PingDataResp{Visits: timedVisits, Logs: logs, SiteLinks: siteLinks}
	ctx.ReturnJSON(resp, 200)
}

func (ctx Ctx) handleLoadComponentsJS() {
	files1, err := filepath.Glob("./static/comp/*.js")
	ctx.CatchError(err)
	files2, err := filepath.Glob("./static/comp/*/*.js")
	ctx.CatchError(err)
	files3, err := filepath.Glob("./static/comp/*/*/*.js")
	ctx.CatchError(err)
	files := append(append(files1, files2...), files3...)
	filesJson, err := json.Marshal(files)
	ctx.CatchError(err)
	ctx.Return(fmt.Sprintf(`
        %s.sort().map(file => {
            let script = document.createElement("script");
            script.src = file.slice(7); script.async = false;
            document.head.appendChild(script)})`, filesJson), 200)
}

func (ctx Ctx) handleUser() {
	ctx.ReturnUser()
}

func (ctx Ctx) handleDump() {
	user := ctx.ForceUser()

	prefsData, err := user.GetPrefs()
	ctx.CatchError(err)

	token, err := user.ReadToken()
	ctx.CatchError(err)

	sitesLink, err := user.GetSiteLinks()
	ctx.CatchError(err)

	sitesDump := make(SitesDump)
	for siteId, count := range sitesLink {
		site := user.NewSite(siteId)
		logs, err := site.GetLogs()
		ctx.CatchError(err)
		visits, err := site.GetVisits(ctx.ParseUTCOffset("utcoffset"))
		ctx.CatchError(err)
		sitesDump[siteId] = SitesDumpVal{
			Logs:   logs,
			Visits: visits,
			Count:  count,
		}
	}

	userDump := UserDump{Id: user.Id, Token: token, Prefs: prefsData}
	dump := Dump{User: userDump, Sites: sitesDump}
	ctx.ReturnJSON(dump, 200)
}

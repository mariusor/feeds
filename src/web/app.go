package main

import (
	"github.com/kataras/iris"
	"github.com/stretchr/gomniauth"
	"github.com/stretchr/objx"
)

func index(ctx iris.Context) {
	// finally, render the template.
	ctx.View("index.html")
}

func login(ctx iris.Context) {
	provider, err := gomniauth.Provider(ctx.Params().Get("provider"))

	if err != nil {
		//
	}

	state := gomniauth.NewState("after", "success")

	// if you want to request additional scopes from the provider,
	// pass them as login?scope=scope1,scope2
	//options := objx.MSI("scope", ctx.QueryValue("scope"))

	authUrl, err := provider.GetBeginAuthURL(state, nil)

	if err != nil {
		//
	}

	// redirect
	ctx.Redirect(authUrl) //goweb.Respond.WithRedirect(ctx, authUrl)
}

func complete(ctx iris.Context) {
	provider, err := gomniauth.Provider(ctx.Params().Get("provider"))

	if err != nil {
		//
	}
	m := objx.MSI(ctx.URLParams())
	creds, err := provider.CompleteAuth(m)

	if err != nil {
		//
	}

	// load the user
	user, userErr := provider.GetUser(creds)

	if userErr != nil {
		//
	}

	ctx.JSON(user)
}

// Serve using a host:port form.
var addr = iris.Addr("localhost:3000")

func main() {
	app := iris.New()
	// Register the templates/**.html as django and reload them on each request
	// so changes can be reflected, set to false on production.
	app.RegisterView(iris.Django("./templates", ".html").Reload(true))

	// GET: http://localhost:3000
	app.Get("/", index)
	app.Get("/auth/{provider:string}/login", login)
	app.Get("/auth/{provider:string}/callback", complete)

	app.OnAnyErrorCode(func(ctx iris.Context) {
		ctx.ViewData("Message", ctx.Values().GetStringDefault("message", "The page you're looking for doesn't exist"))
		ctx.View("error.html")
	})

	// Now listening on: http://localhost:3000
	// Application started. Press CTRL+C to shut down.
	app.Run(addr)
}

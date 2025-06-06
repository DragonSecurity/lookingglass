package main

import (
	"fmt"
	"net/http"

	"github.com/dragonsecurity/lookingglass/internal/version"
	"github.com/justinas/nosurf"
)

func (app *application) newTemplateData(r *http.Request) map[string]any {
	data := map[string]any{
		"CSRFToken": nosurf.Token(r),
		"Version":   version.Get(),
	}
	authenticatedUser, found := contextGetAuthenticatedUser(r)
	if found {
		data["AuthenticatedUser"] = authenticatedUser
	}

	return data
}

func (app *application) newEmailData() map[string]any {
	data := map[string]any{
		"BaseURL": app.config.baseURL,
	}

	return data
}

func (app *application) backgroundTask(r *http.Request, fn func() error) {
	app.wg.Add(1)

	go func() {
		defer app.wg.Done()

		defer func() {
			pv := recover()
			if pv != nil {
				app.reportServerError(r, fmt.Errorf("%v", pv))
			}
		}()

		err := fn()
		if err != nil {
			app.reportServerError(r, err)
		}
	}()
}

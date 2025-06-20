package main

import (
	"log/slog"
	"net/http"
	"os/exec"
	"time"

	"github.com/dragonsecurity/lookingglass/internal/password"
	"github.com/dragonsecurity/lookingglass/internal/request"
	"github.com/dragonsecurity/lookingglass/internal/response"
	"github.com/dragonsecurity/lookingglass/internal/token"
	"github.com/dragonsecurity/lookingglass/internal/validator"

	"github.com/go-chi/chi/v5"
)

func (app *application) home(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)

	err := response.Page(w, http.StatusOK, data, "pages/home.tmpl")
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) signup(w http.ResponseWriter, r *http.Request) {
	var form struct {
		Email     string              `form:"Email"`
		Password  string              `form:"Password"`
		Validator validator.Validator `form:"-"`
	}

	switch r.Method {
	case http.MethodGet:
		data := app.newTemplateData(r)
		data["Form"] = form

		err := response.Page(w, http.StatusOK, data, "pages/signup.tmpl")
		if err != nil {
			app.serverError(w, r, err)
		}

	case http.MethodPost:
		err := request.DecodePostForm(r, &form)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}

		_, found, err := app.db.GetUserByEmail(form.Email)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		form.Validator.CheckField(form.Email != "", "Email", "Email is required")
		form.Validator.CheckField(validator.Matches(form.Email, validator.RgxEmail), "Email", "Must be a valid email address")
		form.Validator.CheckField(!found, "Email", "Email is already in use")

		form.Validator.CheckField(form.Password != "", "Password", "Password is required")
		form.Validator.CheckField(len(form.Password) >= 8, "Password", "Password is too short")
		form.Validator.CheckField(len(form.Password) <= 72, "Password", "Password is too long")
		form.Validator.CheckField(validator.NotIn(form.Password, password.CommonPasswords...), "Password", "Password is too common")

		if form.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = form

			err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/signup.tmpl")
			if err != nil {
				app.serverError(w, r, err)
			}
			return
		}

		hashedPassword, err := password.Hash(form.Password)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		id, err := app.db.InsertUser(form.Email, hashedPassword)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		session, err := app.sessionStore.Get(r, "session")
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		session.Values["userID"] = id

		err = session.Save(r, w)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (app *application) login(w http.ResponseWriter, r *http.Request) {
	var form struct {
		Email     string              `form:"Email"`
		Password  string              `form:"Password"`
		Validator validator.Validator `form:"-"`
	}

	switch r.Method {
	case http.MethodGet:
		data := app.newTemplateData(r)
		data["Form"] = form

		err := response.Page(w, http.StatusOK, data, "pages/login.tmpl")
		if err != nil {
			app.serverError(w, r, err)
		}

	case http.MethodPost:
		err := request.DecodePostForm(r, &form)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}

		user, found, err := app.db.GetUserByEmail(form.Email)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		form.Validator.CheckField(form.Email != "", "Email", "Email is required")
		form.Validator.CheckField(found, "Email", "Email address could not be found")

		if found {
			passwordMatches, err := password.Matches(form.Password, user.HashedPassword)
			if err != nil {
				app.serverError(w, r, err)
				return
			}

			form.Validator.CheckField(form.Password != "", "Password", "Password is required")
			form.Validator.CheckField(passwordMatches, "Password", "Password is incorrect")
		}

		if form.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = form

			err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/login.tmpl")
			if err != nil {
				app.serverError(w, r, err)
			}
			return
		}

		session, err := app.sessionStore.Get(r, "session")
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		session.Values["userID"] = user.ID

		redirectPath, ok := session.Values["redirectPathAfterLogin"].(string)
		if ok {
			delete(session.Values, "redirectPathAfterLogin")
		} else {
			redirectPath = "/"
		}

		err = session.Save(r, w)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		http.Redirect(w, r, redirectPath, http.StatusSeeOther)
	}
}

func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionStore.Get(r, "session")
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	delete(session.Values, "userID")

	err = session.Save(r, w)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) forgottenPassword(w http.ResponseWriter, r *http.Request) {
	var form struct {
		Email     string              `form:"Email"`
		Validator validator.Validator `form:"-"`
	}

	switch r.Method {
	case http.MethodGet:
		data := app.newTemplateData(r)
		data["Form"] = form

		err := response.Page(w, http.StatusOK, data, "pages/forgotten-password.tmpl")
		if err != nil {
			app.serverError(w, r, err)
		}

	case http.MethodPost:
		err := request.DecodePostForm(r, &form)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}

		user, found, err := app.db.GetUserByEmail(form.Email)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		form.Validator.CheckField(form.Email != "", "Email", "Email is required")
		form.Validator.CheckField(validator.Matches(form.Email, validator.RgxEmail), "Email", "Must be a valid email address")
		form.Validator.CheckField(found, "Email", "No matching email found")

		if form.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = form

			err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/forgotten-password.tmpl")
			if err != nil {
				app.serverError(w, r, err)
			}
			return
		}

		plaintextToken := token.New()

		hashedToken := token.Hash(plaintextToken)

		err = app.db.InsertPasswordReset(hashedToken, user.ID, 24*time.Hour)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		data := app.newEmailData()
		data["PlaintextToken"] = plaintextToken

		err = app.mailer.Send(user.Email, data, "forgotten-password.tmpl")
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		http.Redirect(w, r, "/forgotten-password-confirmation", http.StatusSeeOther)
	}
}

func (app *application) forgottenPasswordConfirmation(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)

	err := response.Page(w, http.StatusOK, data, "pages/forgotten-password-confirmation.tmpl")
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) passwordReset(w http.ResponseWriter, r *http.Request) {
	plaintextToken := chi.URLParam(r, "plaintextToken")

	hashedToken := token.Hash(plaintextToken)

	passwordReset, found, err := app.db.GetPasswordReset(hashedToken)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if !found {
		data := app.newTemplateData(r)
		data["InvalidLink"] = true

		err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/password-reset.tmpl")
		if err != nil {
			app.serverError(w, r, err)
		}
		return
	}

	var form struct {
		NewPassword string              `form:"NewPassword"`
		Validator   validator.Validator `form:"-"`
	}

	switch r.Method {
	case http.MethodGet:
		data := app.newTemplateData(r)
		data["Form"] = form
		data["PlaintextToken"] = plaintextToken

		err := response.Page(w, http.StatusOK, data, "pages/password-reset.tmpl")
		if err != nil {
			app.serverError(w, r, err)
		}

	case http.MethodPost:
		err := request.DecodePostForm(r, &form)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}

		form.Validator.CheckField(form.NewPassword != "", "NewPassword", "New password is required")
		form.Validator.CheckField(len(form.NewPassword) >= 8, "NewPassword", "New password is too short")
		form.Validator.CheckField(len(form.NewPassword) <= 72, "NewPassword", "New password is too long")
		form.Validator.CheckField(validator.NotIn(form.NewPassword, password.CommonPasswords...), "NewPassword", "New password is too common")

		if form.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = form
			data["PlaintextToken"] = plaintextToken

			err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/password-reset.tmpl")
			if err != nil {
				app.serverError(w, r, err)
			}
			return
		}

		hashedPassword, err := password.Hash(form.NewPassword)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		err = app.db.UpdateUserHashedPassword(passwordReset.UserID, hashedPassword)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		err = app.db.DeletePasswordResets(passwordReset.UserID)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		http.Redirect(w, r, "/password-reset-confirmation", http.StatusSeeOther)
	}
}

func (app *application) passwordResetConfirmation(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)

	err := response.Page(w, http.StatusOK, data, "pages/password-reset-confirmation.tmpl")
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) protected(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("This is a protected handler"))
}

func (app *application) runHandler(w http.ResponseWriter, r *http.Request) {
	var form struct {
		Command string `form:"command"`
		Target  string `form:"target"`
	}

	switch r.Method {
	case http.MethodGet:
		data := app.newTemplateData(r)
		err := response.Page(w, http.StatusOK, data, "pages/lookingglass.tmpl")
		if err != nil {
			app.serverError(w, r, err)
		}

	case http.MethodPost:
		err := request.DecodePostForm(r, &form)
		if err != nil {
			slog.Error("form decode failed", "err", err)
			app.badRequest(w, r, err)
			return
		}
		// Basic input validation
		validCommands := map[string][]string{
			"ping":       {"-c", "4"},
			"traceroute": {},
			"dig":        {"+short"},
			"whois":      {},
			"mtr":        {"--report-wide", "-c", "4"},
		}

		args, ok := validCommands[form.Command]
		if !ok {
			app.badRequest(w, r, err)
			return
		}

		slog.Debug(form.Command)
		slog.Debug(form.Target)
		// Prevent command injection
		if !validator.SafeHostnameOrIP(form.Target) {
			app.badRequest(w, r, err)
			return
		}

		// Append target to args
		args = append(args, form.Target)
		cmd := exec.Command(form.Command, args...)

		output, err := cmd.CombinedOutput()
		result := string(output)
		if err != nil {
			result += "\n\nError: " + err.Error()
		}

		data := app.newTemplateData(r)
		data["Output"] = result

		err = response.Page(w, http.StatusOK, data, "pages/lookingglass.tmpl")
		if err != nil {
			app.serverError(w, r, err)
		}
	}
}

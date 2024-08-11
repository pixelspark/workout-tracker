package app

import (
	"bytes"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/jovandeginste/workout-tracker/pkg/database"
	"github.com/labstack/echo/v4"
)

func (a *App) routeSegmentsHandler(c echo.Context) error {
	data := a.defaultData(c)

	if err := a.addRouteSegments(data); err != nil {
		return a.redirectWithError(c, a.echo.Reverse("dashboard"), err)
	}

	return c.Render(http.StatusOK, "route_segments_list.html", data)
}

func (a *App) routeSegmentsAddHandler(c echo.Context) error {
	data := a.defaultData(c)
	return c.Render(http.StatusOK, "route_segments_add.html", data)
}

func (a *App) addRouteSegment(c echo.Context) error {
	form, err := c.MultipartForm()
	if err != nil {
		return err
	}

	files := form.File["file"]

	msg := []string{}
	errMsg := []string{}

	for _, file := range files {
		content, parseErr := uploadedFile(file)
		if parseErr != nil {
			errMsg = append(errMsg, parseErr.Error())
			continue
		}

		notes := c.FormValue("notes")

		w, addErr := database.AddRouteSegment(a.db, notes, file.Filename, content)
		if addErr != nil {
			errMsg = append(errMsg, addErr.Error())
			continue
		}

		msg = append(msg, w.Name)
	}

	if len(errMsg) > 0 {
		a.setError(c, "Encountered %d problems while adding route segment: %s", len(errMsg), strings.Join(errMsg, "; "))
	}

	if len(msg) > 0 {
		a.setNotice(c, "Added %d new route segment(s): %s", len(msg), strings.Join(msg, "; "))
	}

	return c.Redirect(http.StatusFound, a.echo.Reverse("route-segments"))
}

func (a *App) routeSegmentsShowHandler(c echo.Context) error {
	data := a.defaultData(c)

	rs, err := a.getRouteSegment(c)
	if err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segments"), err)
	}

	data["routeSegment"] = rs

	return c.Render(http.StatusOK, "route_segments_show.html", data)
}

func (a *App) routeSegmentsDownloadHandler(c echo.Context) error {
	rs, err := a.getRouteSegment(c)
	if err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segments"), err)
	}

	basename := path.Base(rs.Filename)

	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\""+basename+"\"")

	return c.Stream(http.StatusOK, "application/binary", bytes.NewReader(rs.Content))
}

func (a *App) routeSegmentsEditHandler(c echo.Context) error {
	data := a.defaultData(c)

	rs, err := a.getRouteSegment(c)
	if err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segments"), err)
	}

	data["routeSegment"] = rs

	return c.Render(http.StatusOK, "route_segments_edit.html", data)
}

func (a *App) routeSegmentsRefreshHandler(c echo.Context) error {
	rs, err := a.getRouteSegment(c)
	if err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segments"), err)
	}

	if err := rs.UpdateFromContent(); err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segment-show", c.Param("id")), err)
	}

	if err := rs.Save(a.db); err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segment-show", c.Param("id")), err)
	}

	a.setNotice(c, "The workout '%s' has been refreshed.", rs.Name)

	return c.Redirect(http.StatusFound, a.echo.Reverse("route-segment-show", c.Param("id")))
}

func (a *App) routeSegmentsDeleteHandler(c echo.Context) error { //nolint:dupl
	rs, err := a.getRouteSegment(c)
	if err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segments"), err)
	}

	if err := rs.Delete(a.db); err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segment-show", c.Param("id")), err)
	}

	a.setNotice(c, "The workout '%s' has been deleted.", rs.Name)

	return c.Redirect(http.StatusFound, a.echo.Reverse("route-segments"))
}

func (a *App) routeSegmentsUpdateHandler(c echo.Context) error {
	rs, err := a.getRouteSegment(c)
	if err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segments"), err)
	}

	rs.Name = c.FormValue("name")
	rs.Notes = c.FormValue("notes")
	rs.Bidirectional = isChecked(c.FormValue("bidirectional"))
	rs.Circular = isChecked(c.FormValue("circular"))

	if err := rs.Save(a.db); err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segment-edit", c.Param("id")), err)
	}

	a.setNotice(c, "The route segment '%s' has been updated.", rs.Name)

	return c.Redirect(http.StatusFound, a.echo.Reverse("route-segment-show", c.Param("id")))
}

func (a *App) routeSegmentFindMatches(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segment-show", c.Param("id")), err)
	}

	rs, err := database.GetRouteSegment(a.db, id)
	if err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segment-show", c.Param("id")), err)
	}

	db := a.db.Preload("Data.Details").Preload("User")

	w, err := database.GetWorkouts(db)
	if err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segment-show", c.Param("id")), err)
	}

	rs.RouteSegmentMatches = rs.FindMatches(w)
	if err := rs.Save(a.db); err != nil {
		return a.redirectWithError(c, a.echo.Reverse("route-segment-show", c.Param("id")), err)
	}

	a.setNotice(c, "The route segment '%s' has been updated.", rs.Name)

	return c.Redirect(http.StatusFound, a.echo.Reverse("route-segment-show", c.Param("id")))
}

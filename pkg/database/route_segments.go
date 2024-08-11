package database

import (
	"crypto/sha256"
	"html/template"
	"path/filepath"
	"sort"

	"github.com/codingsince1985/geo-golang"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/jovandeginste/workout-tracker/pkg/converters"
	"github.com/microcosm-cc/bluemonday"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RouteSegment struct {
	gorm.Model
	Name          string       `gorm:"not null"` // The name of the workout
	Notes         string       // The notes associated with the workout, in markdown
	Bidirectional bool         // Whether the route segment is bidirectional
	Circular      bool         // Whether the route segment is circular
	GeoAddress    *geo.Address `gorm:"serializer:json"` // The address of the workout
	AddressString string       // The generic location of the workout
	Center        MapCenter    `gorm:"serializer:json"` // The center of the workout (in coordinates)

	TotalDistance float64    // The total distance of the workout
	MinElevation  float64    // The minimum elevation of the workout
	MaxElevation  float64    // The maximum elevation of the workout
	TotalUp       float64    // The total distance up of the workout
	TotalDown     float64    // The total distance down of the workout
	Points        []MapPoint `gorm:"serializer:json"` // The GPS points of the workout

	Content  []byte `gorm:"type:text"`            // The file content
	Checksum []byte `gorm:"not null;uniqueIndex"` // The checksum of the content
	Filename string // The filename of the file

	Dirty               bool                 // Whether the route segment should be recalculated
	RouteSegmentMatches []*RouteSegmentMatch // The matches of the route segment
}

func NewRouteSegment(notes string, filename string, content []byte) (*RouteSegment, error) {
	filename = filepath.Base(filename)

	h := sha256.New()
	h.Write(content)

	rs := &RouteSegment{
		Name:  filename,
		Notes: notes,

		Content:  content,
		Checksum: h.Sum(nil),
		Filename: filename,
	}

	if err := rs.UpdateFromContent(); err != nil {
		return nil, err
	}

	return rs, nil
}

func (rs *RouteSegment) UpdateFromContent() error {
	gpxContent, err := converters.Parse(rs.Filename, rs.Content)
	if err != nil {
		return err
	}

	gpxContent.SimplifyTracks(MaxDeltaMeter / 2)

	data := gpxAsMapData(gpxContent)

	rs.GeoAddress = data.Address
	rs.AddressString = data.AddressString
	rs.Center = data.Center

	rs.TotalDistance = data.TotalDistance
	rs.MinElevation = data.MinElevation
	rs.MaxElevation = data.MaxElevation
	rs.TotalUp = data.TotalUp
	rs.TotalDown = data.TotalDown
	rs.Points = data.Details.Points

	return nil
}

func GetRouteSegment(db *gorm.DB, id int) (*RouteSegment, error) {
	var rs RouteSegment

	if err := db.Preload("RouteSegmentMatches.Workout.User").First(&rs, id).Error; err != nil {
		return nil, err
	}

	sort.Slice(rs.RouteSegmentMatches, func(i, j int) bool {
		return rs.RouteSegmentMatches[i].Workout.GetDate().Before(rs.RouteSegmentMatches[j].Workout.GetDate())
	})

	return &rs, nil
}

func (rs *RouteSegment) Delete(db *gorm.DB) error {
	return db.Unscoped().Select(clause.Associations).Delete(rs).Error
}

func (rs *RouteSegment) Create(db *gorm.DB) error {
	if rs.Content == nil {
		return ErrInvalidData
	}

	return db.Create(rs).Error
}

func (rs *RouteSegment) Save(db *gorm.DB) error {
	if rs.Content == nil {
		return ErrInvalidData
	}

	if rs.RouteSegmentMatches != nil {
		if err := db.Model(rs).Association("RouteSegmentMatches").Replace(rs.RouteSegmentMatches); err != nil {
			return err
		}
	}

	return db.Save(rs).Error
}

func GetRouteSegments(db *gorm.DB) ([]*RouteSegment, error) {
	var rs []*RouteSegment

	if err := db.Preload("RouteSegmentMatches.Workout").Order("created_at DESC").Find(&rs).Error; err != nil {
		return nil, err
	}

	return rs, nil
}

func (rs *RouteSegment) MarkdownNotes() template.HTML {
	doc := parser.NewWithExtensions(parser.CommonExtensions).Parse([]byte(rs.Notes))
	renderer := html.NewRenderer(html.RendererOptions{Flags: html.CommonFlags})
	safeHTML := bluemonday.UGCPolicy().SanitizeBytes(markdown.Render(doc, renderer))

	return template.HTML(safeHTML) //nolint:gosec // We escaped all unsafe HTML with bluemonday
}

func (rs *RouteSegment) Address() string {
	if rs.AddressString != "" {
		return rs.AddressString
	}

	if rs.GeoAddress != nil {
		return rs.GeoAddress.FormattedAddress
	}

	return "(unknown location)"
}

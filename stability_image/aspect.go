package stability_image

import (
	"errors"
	"image"
	"math"
	"sort"
)

type AspectRatio struct {
	Label        string
	Width        float64
	Height       float64
	WidthPixels  uint64
	HeightPixels uint64
	Container    *AspectRatios
}

type AspectRatiosTable map[string]AspectRatio
type ReverseAspectRatiosTable map[image.Point]AspectRatio
type AspectRatioCollection []AspectRatio

type AspectRatios struct {
	MaxPixels     uint64
	MinDimension  uint64
	MaxDimension  uint64
	DimensionStep uint64
	Table         AspectRatiosTable
	ReverseTable  ReverseAspectRatiosTable
}

var (
	DefaultAspectRatiosTable = AspectRatiosTable{
		"21:9": {Label: "21:9", Width: 21, Height: 9},
		"16:9": {Label: "16:9", Width: 16, Height: 9},
		"8:5":  {Label: "8:5", Width: 8, Height: 5},
		"5:4":  {Label: "5:4", Width: 5, Height: 4},
		"4:3":  {Label: "4:3", Width: 4, Height: 3},
		"3:2":  {Label: "3:2", Width: 3, Height: 2},
		"1:1":  {Label: "1:1", Width: 1, Height: 1},
		"2:3":  {Label: "2:3", Width: 2, Height: 3},
		"3:4":  {Label: "3:4", Width: 3, Height: 4},
		"4:5":  {Label: "4:5", Width: 4, Height: 5},
		"5:8":  {Label: "5:8", Width: 5, Height: 8},
		"9:16": {Label: "9:16", Width: 9, Height: 16},
		"9:19": {Label: "9:19", Width: 9, Height: 19},
		"9:21": {Label: "9:21", Width: 9, Height: 21}}
)

func NearestUp(i uint64, x uint64) uint64 {
	remainder := i % x
	if remainder == 0 {
		return i
	}
	return i + x - remainder
}

func Nearest(i uint64, x uint64) uint64 {
	remainder := i % x
	if remainder == 0 {
		return i
	}
	if remainder < x/2 {
		i -= remainder
	} else {
		i += x - remainder
	}
	return i
}

func NearestDown(i uint64, x uint64) uint64 {
	remainder := i % x
	if remainder == 0 {
		return i
	}
	if remainder < x {
		i -= remainder
	} else {
		i += x - remainder
	}
	return i
}

// NearestAspectWH finds the nearest aligned dimensions to the given aspect
// ratio based on the given dimensions and returns the width and height of
func (ar AspectRatios) NearestAspectWH(
	width uint64,
	height uint64,
	totalPixels uint64,
) (uint64, uint64) {
	widthRatio := float64(height) / float64(width)
	heightRatio := float64(width) / float64(height)
	dimSqrd := float64(totalPixels)
	// The usage of `nearestUp`  assumes we have enough VRAM for
	// +DimensionStep pixels in one direction. This allows for more
	// accuracy in the aspect ratio.
	if heightRatio > widthRatio {
		width = Nearest(
			uint64(math.Sqrt(dimSqrd*heightRatio)),
			ar.DimensionStep)
		height = uint64(float64(width) * widthRatio)
	} else {
		height = Nearest(
			uint64(math.Sqrt(dimSqrd*widthRatio)),
			ar.DimensionStep)
		width = uint64(float64(height) * heightRatio)
	}
	if width > height {
		return Nearest(width, ar.DimensionStep),
			Nearest(height, ar.DimensionStep)
	}
	return Nearest(width, ar.DimensionStep),
		Nearest(height, ar.DimensionStep)
}

// NearestAspect finds the nearest aligned dimensions to the given aspect ratio
// based on the given dimensions.
func (ar AspectRatios) NearestAspect(
	aspect string) (
	width uint64,
	height uint64,
	err error) {
	aspectRatio, ok := ar.Table[aspect]
	if !ok {
		return 0, 0, errors.New("invalid aspect ratio")
	}
	width, height = ar.NearestAspectWH(
		uint64(aspectRatio.Width),
		uint64(aspectRatio.Height),
		ar.MaxPixels)
	return width, height, nil
}

// GetDimensions returns the width and height of an AspectRatio based on the
// given total number of pixels.
func (a AspectRatio) GetDimensions(
	totalPixels uint64) (
	width uint64,
	height uint64) {
	if a.WidthPixels != 0 && a.HeightPixels != 0 {
		return a.WidthPixels, a.HeightPixels
	}
	a.WidthPixels, a.HeightPixels = a.Container.NearestAspectWH(uint64(a.Width),
		uint64(a.Height), totalPixels)
	iter := 0
	for ; a.WidthPixels*a.HeightPixels > totalPixels; iter++ {
		a.WidthPixels, a.HeightPixels = a.Container.NearestAspectWH(
			uint64(a.Width),
			uint64(a.Height),
			totalPixels-((32*32)*uint64(iter)))
	}
	return a.WidthPixels, a.HeightPixels
}

// DistancePoint returns the distance between an AspectRatio and an
// image.Point using the Pythagorean theorem.
func (a AspectRatio) DistancePoint(b image.Point) float64 {
	return math.Sqrt(math.Pow(float64(a.WidthPixels)-float64(b.X),
		2) + math.Pow(float64(a.HeightPixels)-float64(b.Y), 2))
}

// GetSortedByNearest accepts an image.Point and a AspectRatiosTable
// returns a slice of AspectRatios sorted by distance from the image.Point.
func (a *AspectRatios) GetSortedByNearest(point image.Point) (
	sorted AspectRatioCollection) {
	sorted = make(AspectRatioCollection, len(a.Table))
	idx := 0
	for _, v := range a.Table {
		sorted[idx] = v
		idx++
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].DistancePoint(point) > sorted[j].DistancePoint(point) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

// LookupAspect takes a width and height, looks up the nearest aspect ratio in
// the ReverseTable and return the string key for that aspect ratio in the
// AspectRatiosTable.
func (a *AspectRatios) LookupAspect(
	width uint64,
	height uint64) (
	aspect *AspectRatio,
	found bool,
) {
	nearest, ok := a.ReverseTable[image.Point{X: int(width),
		Y: int(height)}]
	if ok {
		return &nearest, true
	}
	return nil, false
}

// InsertAspect - given an AspectRatio, insert it into the AspectRatioCollection
// if it is not already in the table.
func (ac *AspectRatioCollection) InsertAspect(aspect *AspectRatio) (
	inserted bool) {
	for _, v := range *ac {
		if v.Label == aspect.Label {
			return false
		}
	}
	*ac = append(*ac, *aspect)
	return true
}

// InsertAspectFilteredByDimensions - given an AspectRatio, insert it into the
// AspectRatioCollection if another AspectRatio does not already have the same
// width and height.
func (ac *AspectRatioCollection) InsertAspectFilteredByDimensions(
	aspect *AspectRatio) (inserted bool) {
	for _, v := range *ac {
		if v.WidthPixels == aspect.WidthPixels &&
			v.HeightPixels == aspect.HeightPixels {
			return false
		}
	}
	*ac = append(*ac, *aspect)
	return true
}

func (ac *AspectRatioCollection) SortByResolution() {
	sort.Slice(*ac, func(i, j int) bool {
		return (*ac)[i].WidthPixels*(*ac)[i].HeightPixels <
			(*ac)[j].WidthPixels*(*ac)[j].HeightPixels
	})
}

func NewAspectRatios(maxPixels uint64, dimensionStep uint64,
	minDimension uint64, maxDimension uint64) AspectRatios {
	table := make(AspectRatiosTable)
	reverse := make(ReverseAspectRatiosTable)
	ars := AspectRatios{
		MaxPixels:     maxPixels,
		MinDimension:  minDimension,
		MaxDimension:  maxDimension,
		DimensionStep: dimensionStep,
		Table:         table,
		ReverseTable:  reverse,
	}
	for _, aspectRatio := range DefaultAspectRatiosTable {
		aspectRatio.Container = &ars
		table[aspectRatio.Label] = aspectRatio
	}

	for k, v := range ars.Table {
		v.WidthPixels, v.HeightPixels = v.GetDimensions(maxPixels)
		if v.WidthPixels >= minDimension && v.HeightPixels >= minDimension &&
			v.WidthPixels <= maxDimension && v.HeightPixels <= maxDimension {
			(ars.Table)[k] = v
			(ars.ReverseTable)[image.Point{X: int(v.WidthPixels),
				Y: int(v.HeightPixels)}] = v
		} else {
			delete(ars.Table, k)
		}
	}
	return ars
}

type AspectOutpaints struct {
	AspectRatio AspectRatio
	Direction   Direction
	Condition   OutpaintCondition
	Outpaints   []OutpaintDescription
}

// FilterByOutpaint - given an AspectRatioCollection and an
// image.Point filter it by the OutpaintCondition and return a slice of
// AspectOutpaints that match the aspect ratio and allowed outpaint conditions.
func (as *AspectRatios) FilterByOutpaint(
	filter image.Point,
	ac AspectRatioCollection,
) (
	filtered []AspectOutpaints,
) {
	filtered = make([]AspectOutpaints, 0)

	for _, aspect := range ac {
		if aspect.WidthPixels < as.MinDimension ||
			aspect.HeightPixels < as.MinDimension ||
			aspect.WidthPixels > as.MaxDimension ||
			aspect.HeightPixels > as.MaxDimension {
			continue
		}
		if aspect.WidthPixels*aspect.HeightPixels > as.MaxPixels {
			continue
		}
		if aspect.WidthPixels == uint64(filter.X) &&
			aspect.HeightPixels == uint64(filter.Y) {
			continue
		}
		descriptions := make([]OutpaintDescription, 0)
		for direction := Direction(0); direction < 5; direction++ {
			outpaintCondition := CondNone
			outpaintCondition.FromPoints(filter, image.Point{
				X: int(aspect.WidthPixels),
				Y: int(aspect.HeightPixels),
			})
			outpaintCondition.SetDirection(direction)

			outpaintAction := outpaintActions[outpaintCondition]

			if outpaintAction != OutpaintNone {
				description := OutpaintDescriptions[outpaintAction]
				description.Condition = outpaintCondition
				descriptions = append(descriptions, description)
			}
		}
		filtered = append(filtered, AspectOutpaints{
			AspectRatio: aspect,
			Outpaints:   descriptions,
		})
	}

	return filtered
}

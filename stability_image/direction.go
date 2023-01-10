package stability_image

import "image"

type Direction int

const (
	DirectionCenter Direction = iota
	DirectionRight
	DirectionLeft
	DirectionDown
	DirectionUp
)

var DirectionMatrix = map[Direction]image.Point{
	DirectionCenter: {0, 0},
	DirectionRight:  {1, 0},
	DirectionLeft:   {-1, 0},
	DirectionUp:     {0, -1},
	DirectionDown:   {0, 1},
}

func (dir Direction) String() string {
	return [...]string{"center", "right", "left", "down", "up"}[dir]
}

func (dir *Direction) FromString(s string) {
	switch s {
	case "center":
		*dir = DirectionCenter
	case "right":
		*dir = DirectionRight
	case "left":
		*dir = DirectionLeft
	case "up":
		*dir = DirectionUp
	case "down":
		*dir = DirectionDown
	}
}

func (dir Direction) IsHorizontal() bool {
	return dir == DirectionRight || dir == DirectionLeft
}

func (dir Direction) IsVertical() bool {
	return dir == DirectionUp || dir == DirectionDown
}

func (dir Direction) IsUpperOrLeft() bool {
	return dir == DirectionUp || dir == DirectionLeft
}

func (dir Direction) IsLowerOrRight() bool {
	return dir == DirectionDown || dir == DirectionRight
}

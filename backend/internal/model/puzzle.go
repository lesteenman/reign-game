package model

// Puzzle represents a grid-based puzzle with colored regions.
// Players place exactly one marker per row, column, and region,
// subject to an adjacency constraint (no two markers touch).
type Puzzle struct {
	ID        string   `json:"puzzleId"`
	GridSize  int      `json:"gridSize"`
	Mode      string   `json:"mode"`
	RegionMap [][]int  `json:"regionMap"`
	Solution  [][]bool `json:"-"` // never serialized to client
}

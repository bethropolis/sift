package app

import (
	"github.com/bethropolis/sift/internal/ignore"
	"github.com/bethropolis/sift/internal/walker"
)

// walkDirectory is a helper method that performs the actual directory walk
func (a *App) walkDirectory(
	rootDir string,
	matcher *ignore.IgnoreMatcher,
	walkFn walker.WalkFunc,
	options []walker.Option,
) ([]walker.SkippedItem, error) {
	return walker.Walk(rootDir, matcher, walkFn, options...)
}

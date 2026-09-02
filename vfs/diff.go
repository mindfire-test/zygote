package vfs

// Diff compares two world snapshots and returns a list of changes.
// It performs an O(diff) comparison by skipping subtrees with identical hashes.
func Diff(s Store, from, to Snapshot) ([]Change, error) {
	if from.Root == to.Root {
		return nil, nil
	}
	return diffTree(s, "", from.Root, to.Root)
}

func diffTree(s Store, prefix string, fromHash, toHash Hash) ([]Change, error) {
	if fromHash == toHash {
		return nil, nil // hashes match, subtree is identical
	}

	var changes []Change

	fromTree, err := loadTree(s, fromHash)
	if err != nil {
		return nil, err
	}
	toTree, err := loadTree(s, toHash)
	if err != nil {
		return nil, err
	}

	fromMap := make(map[string]Entry)
	for _, e := range fromTree {
		fromMap[e.Name] = e
	}

	toMap := make(map[string]Entry)
	for _, e := range toTree {
		toMap[e.Name] = e
	}

	// Check for deleted or modified
	for name, fe := range fromMap {
		path := name
		if prefix != "" {
			path = prefix + "/" + name
		}

		if te, ok := toMap[name]; ok {
			// Modified?
			if fe.Hash != te.Hash {
				if fe.Kind == KindDir && te.Kind == KindDir {
					subChanges, err := diffTree(s, path, fe.Hash, te.Hash)
					if err != nil {
						return nil, err
					}
					changes = append(changes, subChanges...)
				} else {
					changes = append(changes, Change{
						Path: path,
						Kind: Modified,
						From: fe.Hash,
						To:   te.Hash,
					})
				}
			}
		} else {
			// Deleted
			delChanges, err := walkAndMark(s, path, fe, Deleted)
			if err != nil {
				return nil, err
			}
			changes = append(changes, delChanges...)
		}
	}

	// Check for added
	for name, te := range toMap {
		path := name
		if prefix != "" {
			path = prefix + "/" + name
		}

		if _, ok := fromMap[name]; !ok {
			addChanges, err := walkAndMark(s, path, te, Added)
			if err != nil {
				return nil, err
			}
			changes = append(changes, addChanges...)
		}
	}

	return changes, nil
}

func walkAndMark(s Store, path string, e Entry, kind ChangeKind) ([]Change, error) {
	if e.Kind == KindFile {
		change := Change{
			Path: path,
			Kind: kind,
		}
		if kind == Added {
			change.To = e.Hash
		} else {
			change.From = e.Hash
		}
		return []Change{change}, nil
	}

	t, err := loadTree(s, e.Hash)
	if err != nil {
		return nil, err
	}

	var changes []Change
	for _, subE := range t {
		subPath := path + "/" + subE.Name
		subChanges, err := walkAndMark(s, subPath, subE, kind)
		if err != nil {
			return nil, err
		}
		changes = append(changes, subChanges...)
	}
	return changes, nil
}

func loadTree(s Store, h Hash) (Tree, error) {
	if h == EmptyTreeHash {
		return Tree{}, nil
	}
	b, ok := s.Get(h)
	if !ok {
		// Should we return error or just say not found?
		return nil, ErrNotFound
	}
	return DecodeTree(b)
}

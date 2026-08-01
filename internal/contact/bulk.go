package contact

type BulkCreateRequest struct {
	Contacts []BulkContact `json:"contacts"`
}

type BulkContact struct {
	Name     string   `json:"name"`
	Email    string   `json:"email,omitempty"`
	Phone    string   `json:"phone,omitempty"`
	Location string   `json:"location,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type BulkCreateResponse struct {
	Imported int            `json:"imported"`
	Failed   int            `json:"failed"`
	Errors   []BulkRowError `json:"errors,omitempty"`
}

type BulkRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

func (s *Service) bulkCreate(req BulkCreateRequest) (*BulkCreateResponse, error) {
	resp := &BulkCreateResponse{}
	for i, c := range req.Contacts {
		if c.Name == "" {
			resp.Failed++
			resp.Errors = append(resp.Errors, BulkRowError{Row: i + 1, Message: "name is required"})
			continue
		}
		var tagIDs []string
		for _, tagName := range c.Tags {
			var tagID string
			err := s.db.QueryRow(`SELECT id FROM tags WHERE name = $1 AND type = 'tag'`, tagName).Scan(&tagID)
			if err == nil {
				tagIDs = append(tagIDs, tagID)
			}
		}
		_, err := s.create(CreateRequest{
			Name:     c.Name,
			Email:    c.Email,
			Phone:    c.Phone,
			Location: c.Location,
			TagIDs:   tagIDs,
		})
		if err != nil {
			resp.Failed++
			resp.Errors = append(resp.Errors, BulkRowError{Row: i + 1, Message: err.Error()})
			continue
		}
		resp.Imported++
	}
	return resp, nil
}

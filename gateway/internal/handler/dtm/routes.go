package dtm

import "github.com/cloudwego/hertz/pkg/app/server"

func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/internal/dtm/workflow", WorkflowCallback())
}

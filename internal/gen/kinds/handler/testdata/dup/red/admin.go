package plansapi

// ArchivePlan serves POST /plans/{id}/archive.
func (h *Handler) ArchivePlan(ctx context.Context, request apigen.ArchivePlanRequestObject) (apigen.ArchivePlanResponseObject, error) {
	_, err := h.uc.ArchivePlan.Execute(ctx, growth.AccountID(request.Id))
	if err == nil {
		return apigen.ArchivePlan204Response{}, nil
	}
	status := http.StatusInternalServerError
	rules := []statusRule{
		{err: growth.ErrPlanNotFound, status: http.StatusNotFound},
		{err: growth.ErrPlanArchived, status: http.StatusConflict},
		{err: growth.ErrPlanLocked, status: http.StatusLocked},
	}
	for _, rule := range rules {
		if errors.Is(err, rule.err) {
			status = rule.status
			break
		}
	}
	detail := err.Error()
	problem := apigen.Problem{Title: http.StatusText(status), Status: int32(status), Detail: &detail}
	if status == http.StatusNotFound {
		return apigen.ArchivePlan404ApplicationProblemPlusJSONResponse(problem), nil
	}
	return nil, fmt.Errorf("plansapi: archive plan: %w", err)
}

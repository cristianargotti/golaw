package plansapi

// GetPlan serves GET /plans/{id}.
func (h *Handler) GetPlan(ctx context.Context, request apigen.GetPlanRequestObject) (apigen.GetPlanResponseObject, error) {
	plan, err := h.uc.ReadPlan.Execute(ctx, growth.AccountID(request.Id))
	if err == nil {
		return apigen.GetPlan200JSONResponse{Account: string(plan.Account), Name: plan.Name, Archived: plan.Archived}, nil
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
		return apigen.GetPlan404ApplicationProblemPlusJSONResponse(problem), nil
	}
	return nil, fmt.Errorf("plansapi: get plan: %w", err)
}

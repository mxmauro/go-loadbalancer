package loadbalancer

// -----------------------------------------------------------------------------

func (lb *LoadBalancer) raiseEvent(eventType int, server *Server) {
	lb.eventHandlerMtx.RLock()
	if lb.eventHandler != nil {
		lb.eventHandler(eventType, server)
	}
	lb.eventHandlerMtx.RUnlock()
}


package main

//type target struct {
//	Target string `json:"target"`
//}
//
//type xstate struct {
//	Id string `json:"id"`
//	Initial string `json:"initial,omitempty"`
//	States map[string]*xstate `json:"states,omitempty"`
//	Transitions map[string][]target `json:"on,omitempty"`
//}
//
//func (s StateNode) ToString() (string, error) {
//	out, err := s.ToXState()
//	if err != nil {
//		return "", err
//	}
//	bytes, err := json.Marshal(out)
//	if err != nil {
//		return "", err
//	}
//	return string(bytes), nil
//}
//
//func (s StateNode) ToXState() (*xstate, error) {
//	out := &xstate{
//		Id: "."+s.Name,
//	}
//	if s.InitialState != nil {
//		out.Initial = s.InitialState.Name
//	}
//	if len(s.States) > 0 {
//		out.States = make(map[string]*xstate)
//	}
//	for _, child := range s.States {
//		var err error
//		out.States[child.Name], err = child.ToXState()
//		if err != nil {
//			return nil, err
//		}
//	}
//
//	if len(s.Transitions) > 0 {
//		out.Transitions = make(map[string][]target)
//	}
//	for e, ts := range s.Transitions {
//		targets := []target{}
//		for _, t := range ts {
//			targets = append(targets, target{Target: "."+t.Target.Name})
//		}
//		out.Transitions[string(e)] = targets
//	}
//	return out, nil
//}
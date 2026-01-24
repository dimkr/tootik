/*
Copyright 2025, 2026 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ap

import (
	"errors"
	"fmt"
	"strings"
)

func ValidateOrigin(domain string, activity *Activity, origin string) error {
	return validateOrigin(domain, activity, origin, 0)
}

func validateOrigin(domain string, activity *Activity, origin string, depth uint) error {
	if depth == MaxActivityDepth {
		return errors.New("activity is too nested")
	}

	if origin == domain {
		return errors.New("invalid origin")
	}

	if activity.ID == "" {
		return errors.New("unspecified activity ID")
	}

	activityOrigin, err := Origin(activity.ID)
	if err != nil {
		return err
	}

	if activityOrigin != origin {
		return fmt.Errorf("invalid activity host: %s", activityOrigin)
	}

	if activity.Actor == "" {
		return errors.New("unspecified actor")
	}

	actorOrigin, err := Origin(activity.Actor)
	if err != nil {
		return err
	}

	if actorOrigin != origin {
		return fmt.Errorf("invalid actor host: %s", actorOrigin)
	}

	switch activity.Type {
	case Delete:
		// $origin can only delete objects that belong to $origin
		switch v := activity.Object.(type) {
		case *Object:
			if objectOrigin, err := Origin(v.ID); err != nil {
				return err
			} else if objectOrigin != origin {
				return fmt.Errorf("invalid object host: %s", objectOrigin)
			}

		case string:
			if stringOrigin, err := Origin(v); err != nil {
				return err
			} else if stringOrigin != origin {
				return fmt.Errorf("invalid object host: %s", stringOrigin)
			}

		default:
			return fmt.Errorf("invalid object: %T", v)
		}

	case Follow:
		if inner, ok := activity.Object.(string); ok {
			if _, err := Origin(inner); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("invalid object: %T", activity.Object)
		}

	case Accept, Reject:
		// $origin can only accept or reject Follow activities that belong to us
		switch v := activity.Object.(type) {
		case *Activity:
			if v.Type != Follow {
				return fmt.Errorf("invalid object type: %s", v.Type)
			}

			if innerOrigin, err := Origin(v.ID); err != nil {
				return err
			} else if innerOrigin != domain && !strings.HasPrefix(innerOrigin, "did:") {
				return fmt.Errorf("invalid object host: %s", innerOrigin)
			}

		case string:
			if innerOrigin, err := Origin(v); err != nil {
				return err
			} else if innerOrigin != domain && !strings.HasPrefix(innerOrigin, "did:") {
				return fmt.Errorf("invalid object host: %s", innerOrigin)
			}

		default:
			return fmt.Errorf("invalid object: %T", v)
		}

	case Undo:
		if inner, ok := activity.Object.(*Activity); ok {
			if inner.Type != Announce && inner.Type != Follow {
				return fmt.Errorf("invalid inner activity: %w: %s", ErrUnsupportedActivity, inner.Type)
			}

			// $origin can only undo actions performed by actors from $origin
			if err := validateOrigin(domain, inner, origin, depth+1); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("invalid object: %T", activity.Object)
		}

	case Create, Update:
		// $origin can only create objects that belong to $origin
		if obj, ok := activity.Object.(*Object); ok {
			if objectOrigin, err := Origin(obj.ID); err != nil {
				return err
			} else if objectOrigin != origin {
				return fmt.Errorf("invalid object host: %s", objectOrigin)
			} else if obj.AttributedTo != "" && obj.AttributedTo != activity.Actor {
				authorOrigin, err := Origin(obj.AttributedTo)
				if err != nil {
					return err
				}

				if authorOrigin != origin {
					return fmt.Errorf("invalid author host: %s", authorOrigin)
				}
			}
		} else if s, ok := activity.Object.(string); ok {
			if stringOrigin, err := Origin(s); err != nil {
				return err
			} else if stringOrigin != origin {
				return fmt.Errorf("invalid object host: %s", stringOrigin)
			}
		} else {
			return fmt.Errorf("invalid object: %T", obj)
		}

	case Announce:
		// we always unwrap nested Announce, validate the inner activity and don't allow nesting
		if _, ok := activity.Object.(*Activity); ok {
			return errors.New("announce must not be nested")
		} else if s, ok := activity.Object.(string); !ok {
			return fmt.Errorf("invalid object: %T", activity.Object)
		} else if s == "" {
			return errors.New("empty ID")
		} else if _, err := Origin(s); err != nil {
			return err
		}

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedActivity, activity.Type)
	}

	return nil
}

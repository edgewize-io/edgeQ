package query

import (
	"sort"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

type Processor struct {
	q              *Query
	compareFunc    CompareFunc
	filterFunc     FilterFunc
	transformFuncs []TransformFunc
}

// CompareFunc return true is left great than right
type CompareFunc func(interface{}, interface{}, Field) bool

type FilterFunc func(interface{}, Filter) bool

type TransformFunc func(interface{}) interface{}

func NewDefaultProcessor(q *Query) *Processor {
	compareFunc := func(left, right interface{}, field Field) bool {
		leftAccessor, ok := left.(v1.ObjectMetaAccessor)
		if !ok {
			return false
		}

		rightAccessor, ok := right.(v1.ObjectMetaAccessor)
		if !ok {
			return true
		}
		return DefaultObjectMetaCompare(leftAccessor.GetObjectMeta(), rightAccessor.GetObjectMeta(), field)
	}

	filterFunc := func(item interface{}, filter Filter) bool {
		accessor, ok := item.(v1.ObjectMetaAccessor)
		if !ok {
			return false
		}

		return DefaultObjectMetaFilter(accessor.GetObjectMeta(), filter)
	}

	return &Processor{
		q:           q,
		compareFunc: compareFunc,
		filterFunc:  filterFunc,
	}
}

func (p *Processor) SetCompareFunc(compareFunc CompareFunc) *Processor {
	p.compareFunc = compareFunc
	return p
}

func (p *Processor) SetFilterFunc(filterFunc FilterFunc) *Processor {
	p.filterFunc = filterFunc
	return p
}

func (p *Processor) SetTransformFunc(transformFuncs ...TransformFunc) *Processor {
	p.transformFuncs = transformFuncs
	return p
}

func (p *Processor) FilterAndSort(objects []interface{}) []interface{} {
	// selected matched ones
	var filtered []interface{}
	for _, object := range objects {
		selected := true
		for field, value := range p.q.Filters {
			if !p.filterFunc(object, Filter{Field: field, Value: value}) {
				selected = false
				break
			}
		}

		if selected {
			for _, transform := range p.transformFuncs {
				object = transform(object)
			}
			filtered = append(filtered, object)
		}
	}

	// sort by sortBy field
	sort.Slice(filtered, func(i, j int) bool {
		if !p.q.Ascending {
			return p.compareFunc(filtered[i], filtered[j], p.q.SortBy)
		}
		return !p.compareFunc(filtered[i], filtered[j], p.q.SortBy)
	})

	return filtered
}

func (p *Processor) Pagination(objects []interface{}) []interface{} {
	if p.q.Pagination == nil {
		p.q.Pagination = NoPagination
	}

	start, end := p.q.Pagination.GetValidPagination(len(objects))

	return objects[start:end]
}

// DefaultObjectMetaCompare return true is left great than right
func DefaultObjectMetaCompare(left, right v1.Object, sortBy Field) bool {
	switch sortBy {
	// ?sortBy=name
	case FieldName:
		return strings.Compare(left.GetName(), right.GetName()) > 0
	//	?sortBy=namespace
	case FieldNamespace:
		return strings.Compare(left.GetNamespace(), right.GetNamespace()) > 0
	default:
		fallthrough
	//	?sortBy=creationTimestamp
	case FieldCreationTimeStamp:
		// compare by name if creation timestamp is equal
		if left.GetCreationTimestamp() == right.GetCreationTimestamp() {
			return strings.Compare(left.GetName(), right.GetName()) > 0
		}
		return left.GetCreationTimestamp().After(right.GetCreationTimestamp().Time)
	}
}

func DefaultObjectMetaFilter(item v1.Object, filter Filter) bool {
	switch filter.Field {
	case FieldNames:
		for _, name := range strings.Split(string(filter.Value), ",") {
			if item.GetName() == name {
				return true
			}
		}
		return false
	// /namespaces?page=1&limit=10&name=default
	case FieldName:
		return strings.Contains(item.GetName(), string(filter.Value))
	// /clusters?page=1&limit=10&alias=xxx
	case FieldNamespace:
		return strings.Compare(item.GetNamespace(), string(filter.Value)) == 0
	// /namespaces?page=1&limit=10&label=kubesphere.io/workspace:system-workspace
	case FieldLabel:
		return labelMatch(item.GetLabels(), string(filter.Value))
	default:
		return true
	}
}

func labelMatch(m map[string]string, filter string) bool {
	labelSelector, err := labels.Parse(filter)
	if err != nil {
		klog.Warningf("invalid labelSelector %s: %s", filter, err)
		return false
	}
	return labelSelector.Matches(labels.Set(m))
}

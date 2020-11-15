/*
 * Copyright 2020 Valstack Info Pvt Ltd,India.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package route

import (
	"errors"
	"net/url"
	"strings"
)

type Route struct {
	IsPrivate bool
	// Any http method. It will be used as uppercase to avoid common mistakes.
	HttpMethod string
	// A string like "/resource/:id.json".
	// Placeholders supported are:
	// :param that matches any char to the first '/' or '.'
	// *splat that matches everything to the end of the string
	// (placeholder names should be unique per PathExp)
	PathExp string

	// Code that will be executed when this route is taken.
	// Func http.HandlerFunc
	Func interface{}
}

type Router struct {
	routes                 []Route
	disableTrieCompression bool
	index                  map[*Route]int
	trie                   *Trie
}

// Define the Routes. The order the Routes matters,
// if a request matches multiple Routes, the first one will be used.
func (self *Router) SetRoutes(routes ...Route) error {

	self.routes = routes
	err := self.start()

	if err != nil {
		return err
	}

	return nil
}

func escapedPath(urlObj *url.URL) string {
	// the escape method of url.URL should be public
	// that would avoid this split.
	parts := strings.SplitN(urlObj.RequestURI(), "?", 2)
	return parts[0]
}

// This validates the Routes and prepares the Trie data structure.
// It must be called once the Routes are defined and before trying to find Routes.
// The order matters, if multiple Routes match, the first defined will be used.
func (router *Router) start() error {

	router.trie = NewTrie()
	router.index = map[*Route]int{}

	for i := range router.routes {

		// pointer to the Route
		route := &router.routes[i]

		// PathExp validation
		if route.PathExp == "" {
			return errors.New("empty PathExp")
		}
		if route.PathExp[0] != '/' {
			return errors.New("PathExp must start with /")
		}
		urlObj, err := url.Parse(route.PathExp)
		if err != nil {
			return err
		}

		// work with the PathExp urlencoded.
		pathExp := escapedPath(urlObj)

		// make an exception for '*' used by the *splat notation
		// (at the trie insert only)
		pathExp = strings.Replace(pathExp, "%2A", "*", -1)

		// insert in the Trie
		err = router.trie.AddRoute(
			strings.ToUpper(route.HttpMethod), // work with the HttpMethod in uppercase
			pathExp,
			route,
		)
		if err != nil {
			return err
		}

		// index
		router.index[route] = i
	}

	if router.disableTrieCompression == false {
		router.trie.Compress()
	}

	return nil
}

// return the result that has the route defined the earliest
func (self *Router) ofFirstDefinedRoute(matches []*Match) *Match {
	minIndex := -1
	matchesByIndex := map[int]*Match{}

	for _, result := range matches {
		route := result.Route.(*Route)
		routeIndex := self.index[route]
		matchesByIndex[routeIndex] = result
		if minIndex == -1 || routeIndex < minIndex {
			minIndex = routeIndex
		}
	}

	return matchesByIndex[minIndex]
}
func (self *Router) AddRoute(route Route) error {
	err := self.trie.AddRoute(
		strings.ToUpper(route.HttpMethod), // work with the HttpMethod in uppercase
		route.PathExp,
		&route,
	)
	if err != nil {
		return err
	}
	return nil
}

// Return the first matching Route and the corresponding parameters for a given URL object.
func (self *Router) FindRouteFromURL(httpMethod string, urlObj *url.URL) (*Route, map[string]string, bool) {

	// lookup the routes in the Trie
	matches, pathMatched := self.trie.FindRoutesAndPathMatched(
		strings.ToUpper(httpMethod), // work with the httpMethod in uppercase
		escapedPath(urlObj),         // work with the path urlencoded
	)

	// short cuts
	if len(matches) == 0 {
		// no route found
		return nil, nil, pathMatched
	}

	if len(matches) == 1 {
		// one route found
		return matches[0].Route.(*Route), matches[0].Params, pathMatched
	}

	// multiple routes found, pick the first defined
	result := self.ofFirstDefinedRoute(matches)
	return result.Route.(*Route), result.Params, pathMatched
}

// Parse the url string (complete or just the path) and return the first matching Route and the corresponding parameters.
func (self *Router) FindRoute(httpMethod, urlStr string) (*Route, map[string]string, bool, error) {

	// parse the url
	urlObj, err := url.Parse(urlStr)
	if err != nil {
		return nil, nil, false, err
	}

	route, params, pathMatched := self.FindRouteFromURL(httpMethod, urlObj)
	return route, params, pathMatched, nil
}

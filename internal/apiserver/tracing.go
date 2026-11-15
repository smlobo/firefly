// Copyright © 2026 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apiserver

import (
	"context"

	"github.com/hyperledger-firefly/common/pkg/ffapi"
	"github.com/hyperledger-firefly/firefly/internal/telemetry"
	"github.com/hyperledger-firefly/firefly/pkg/core"
	"go.opentelemetry.io/otel/attribute"
)

func traceAPIRequest(r *ffapi.APIRequest, routeName string, invoke func(context.Context) (interface{}, error)) (output interface{}, err error) {
	ctx, span := telemetry.StartServerSpan(r.Req.Context(), r.Req.Header, "firefly.api."+routeName,
		attribute.String("http.request.method", r.Req.Method),
		attribute.String("firefly.api.route", routeName),
	)
	defer func() { telemetry.EndSpan(span, err) }()

	output, err = invoke(ctx)
	if err != nil {
		return output, err
	}
	span.SetAttributes(attribute.Int("http.response.status_code", r.SuccessStatus))
	if op, ok := output.(*core.Operation); ok && op != nil && op.ID != nil {
		span.SetAttributes(attribute.String("firefly.operation.id", op.ID.String()))
	}
	return output, nil
}

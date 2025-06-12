/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as fm from "../../fetch.pb"
export type DummyRequest = {
}

export type DummyResponse = {
}

export class ClusterMonitorService {
  static Dummy(req: DummyRequest, initReq?: fm.InitReq): Promise<DummyResponse> {
    return fm.fetchReq<DummyRequest, DummyResponse>(`/v1/dummy?${fm.renderURLSearchParams(req, [])}`, {...initReq, method: "GET"})
  }
}
/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as fm from "../../fetch.pb"

type Absent<T, K extends keyof T> = { [k in Exclude<keyof T, K>]?: undefined };
type OneOf<T> =
  | { [k in keyof T]?: undefined }
  | (
    keyof T extends infer K ?
      (K extends string & keyof T ? { [k in K]: T[K] } & Absent<T, K>
        : never)
    : never);

type BaseSendClusterTelemetryRequestPayload = {
}

export type SendClusterTelemetryRequestPayload = BaseSendClusterTelemetryRequestPayload
  & OneOf<{ cluster_snapshot: ClusterSnapshot }>

export type SendClusterTelemetryRequest = {
  payloads?: SendClusterTelemetryRequestPayload[]
}

export type ClusterSnapshotNodeNvidiaAttributes = {
  product?: string
}

export type ClusterSnapshotNode = {
  name?: string
  gpu_capacity?: number
  memory_capacity?: string
  nvidia_attributes?: ClusterSnapshotNodeNvidiaAttributes
}

export type ClusterSnapshot = {
  nodes?: ClusterSnapshotNode[]
}

export type SendClusterTelemetryResponse = {
}

export class ClusterMonitorWorkerService {
  static SendClusterTelemetry(req: SendClusterTelemetryRequest, initReq?: fm.InitReq): Promise<SendClusterTelemetryResponse> {
    return fm.fetchReq<SendClusterTelemetryRequest, SendClusterTelemetryResponse>(`/llmariner.clustermonitor.server.v1.ClusterMonitorWorkerService/SendClusterTelemetry`, {...initReq, method: "POST", body: JSON.stringify(req)})
  }
}
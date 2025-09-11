import * as fm from "../../fetch.pb";
type Absent<T, K extends keyof T> = {
    [k in Exclude<keyof T, K>]?: undefined;
};
type OneOf<T> = {
    [k in keyof T]?: undefined;
} | (keyof T extends infer K ? (K extends string & keyof T ? {
    [k in K]: T[K];
} & Absent<T, K> : never) : never);
type BaseSendClusterTelemetryRequestPayload = {};
export type SendClusterTelemetryRequestPayload = BaseSendClusterTelemetryRequestPayload & OneOf<{
    cluster_snapshot: ClusterSnapshot;
    gpu_telemetry: GpuTelemetry;
}>;
export type SendClusterTelemetryRequest = {
    payloads?: SendClusterTelemetryRequestPayload[];
};
export type ClusterSnapshotNodeNvidiaAttributes = {
    product?: string;
};
export type ClusterSnapshotNode = {
    name?: string;
    gpu_capacity?: number;
    memory_capacity?: string;
    nvidia_attributes?: ClusterSnapshotNodeNvidiaAttributes;
    gpu_occupancy?: number;
    pod_count?: number;
};
export type ClusterSnapshot = {
    nodes?: ClusterSnapshotNode[];
};
export type GpuTelemetryNode = {
    name?: string;
    max_gpu_used?: number;
    avg_gpu_used?: number;
    max_gpu_memory_used?: string;
    avg_gpu_memory_used?: string;
};
export type GpuTelemetry = {
    nodes?: GpuTelemetryNode[];
};
export type SendClusterTelemetryResponse = {};
export declare class ClusterMonitorWorkerService {
    static SendClusterTelemetry(req: SendClusterTelemetryRequest, initReq?: fm.InitReq): Promise<SendClusterTelemetryResponse>;
}
export {};

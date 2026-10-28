/** Severity of a problem, as the Go side names it. */
export type Severity = 'INFO' | 'WARNING' | 'ERROR';

/** CommandStatus: unknown means the command may or may not have been applied. */
export type CommandStatus = 'queued' | 'dispatched' | 'applied' | 'failed' | 'expired' | 'unknown';

/** Problem is one message about a site; a node problem has no pod. */
export interface Problem {
  key: string;
  severity: Severity;
  outcome: string;
  reason: string;
  rule: string;
  title: string;
  text: string;
  target: {
    namespace: string;
    pod: string;
    container: string;
    node?: string;
    ownerKind?: string;
    ownerName?: string;
    ownerUID?: string;
  };
  details?: { key: string; value: string }[] | null;
  time: string;
}

/** Site is the last known state of a site. */
export interface Site {
  id: string;
  last_contact: string;
  last_success: string;
  stale: boolean;
  last_error: string;
  messages: Problem[];
}

/** Observation is one stored snapshot of a site. */
export interface Observation {
  id: string;
  site_id: string;
  observed_at: string;
  collected: boolean;
  error?: string;
  messages: Problem[] | null;
}
/** CommandKind is what a command does. */
export type CommandKind = 'restart_deployment' | 'set_resources' | 'scale';
/** TargetKind is the workload a command changes. */
export type TargetKind = 'Deployment' | 'StatefulSet' | 'DaemonSet';

/** Resources holds Kubernetes quantities such as 500m or 256Mi; an omitted one is left unchanged. */
export interface Resources {
  cpu_request?: string;
  cpu_limit?: string;
  memory_request?: string;
  memory_limit?: string;
}

/** Command is a deferred change of one workload. */
export interface Command {
  id: string;
  site_id: string;
  kind: CommandKind;
  target_kind?: TargetKind;
  namespace: string;
  name: string;
  target_uid: string;
  container?: string;
  resources?: Resources;
  replicas?: number;
  created_at: string;
  expires_at: string;
  status: CommandStatus;
  note?: string;
}

/** CreateCommand is what the operator sends; the center fills in the rest. */
export type CreateCommand = Pick<
  Command,
  | 'id'
  | 'site_id'
  | 'kind'
  | 'target_kind'
  | 'namespace'
  | 'name'
  | 'target_uid'
  | 'container'
  | 'resources'
  | 'replicas'
  | 'expires_at'
>;

/** DemoLink is the state of the simulated agent link. */
export interface DemoLink {
  connected: boolean;
  simulated: boolean;
}

/** APIError is a failed request as the UI handles it. */
export interface APIError {
  status: number | 'FETCH_ERROR' | 'TIMEOUT_ERROR' | 'PARSING_ERROR';
  message: string;
}

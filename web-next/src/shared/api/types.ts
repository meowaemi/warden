export type Severity = 'INFO' | 'WARNING' | 'ERROR';
export type CommandStatus = 'queued' | 'dispatched' | 'applied' | 'failed' | 'expired' | 'unknown';
export interface Problem {
  key: string;
  severity: Severity;
  outcome: string;
  reason: string;
  rule: string;
  title: string;
  text: string;
  target: { namespace: string; pod: string; container: string; node?: string };
  details?: { key: string; value: string }[] | null;
  time: string;
}
export interface Site {
  id: string;
  last_contact: string;
  last_success: string;
  stale: boolean;
  last_error: string;
  messages: Problem[];
}
export interface Observation {
  id: string;
  site_id: string;
  observed_at: string;
  collected: boolean;
  error?: string;
  messages: Problem[] | null;
}
export interface Command {
  id: string;
  site_id: string;
  kind: 'restart_deployment';
  namespace: string;
  name: string;
  target_uid: string;
  created_at: string;
  expires_at: string;
  status: CommandStatus;
  note?: string;
}
export type CreateCommand = Pick<
  Command,
  'id' | 'site_id' | 'kind' | 'namespace' | 'name' | 'target_uid' | 'expires_at'
>;
export interface DemoLink {
  connected: boolean;
  simulated: boolean;
}
export interface APIError {
  status: number | 'FETCH_ERROR' | 'TIMEOUT_ERROR' | 'PARSING_ERROR';
  message: string;
}

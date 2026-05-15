export interface VisaType {
  code: string;
  display_name_en: string;
  display_name_ja: string;
}

export interface Visa {
  id: number;
  user_id: number;
  visa_type_code: string;
  status: string;
  coe_number?: string;
  coe_issued_at?: string;
  landed_at?: string;
  residence_card_number?: string;
  residence_card_issued_at?: string;
  period_of_stay_months?: number;
  expires_at?: string;
  sponsor_name?: string;
  sponsor_address?: string;
  job_title?: string;
  notes?: string;
  created_at: string;
  updated_at: string;
}

export interface Task {
  id: number;
  user_id: number;
  visa_id?: number;
  rule_id: string;
  triggered_by_event_id?: number;
  title_en: string;
  title_ja?: string;
  description_en: string;
  description_ja?: string;
  category: string;
  severity: string;
  status: string;
  deadline_at?: string;
  completed_at?: string;
  legal_source_url?: string;
  legal_source_text?: string;
  location_hint?: string;
}

export interface LifeEventResponse {
  event: {
    id: number;
    user_id: number;
    visa_id?: number;
    event_type: string;
    occurred_at: string;
    payload: unknown;
    created_at: string;
  };
  tasks: Task[];
}

export type EventType =
  | "visa_application_started"
  | "visa_applied"
  | "visa_approved"
  | "coe_received"
  | "landed_japan"
  | "address_registered"
  | "address_changed"
  | "employer_changed"
  | "visa_renewal_window_opens"
  | "tax_residency_triggered";

export const EVENT_TYPE_LABELS: Record<EventType, string> = {
  visa_application_started: "Started visa application",
  visa_applied: "Applied for visa",
  visa_approved: "Visa approved",
  coe_received: "Received CoE",
  landed_japan: "Landed in Japan",
  address_registered: "Registered address",
  address_changed: "Changed address",
  employer_changed: "Changed employer",
  visa_renewal_window_opens: "Visa renewal window opened",
  tax_residency_triggered: "Tax residency triggered (183+ days)",
};

export interface ConversationMessage {
  role: "user" | "assistant";
  content: string;
}

export interface ConversationToolCall {
  name: string;
  success: boolean;
  message: string;
  data?: unknown;
}

export interface ConversationResponse {
  message: string;
  tool_calls?: ConversationToolCall[];
  response_id?: string;
  model?: string;
}

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
  const body = await res.json();
  if (!res.ok) {
    throw new ApiError(res.status, body.error ?? res.statusText);
  }
  return body as T;
}

export const api = {
  listVisaTypes: () =>
    request<{ visa_types: VisaType[] }>("/api/v1/visa-types").then(
      (r) => r.visa_types,
    ),

  createVisa: (body: {
    visa_type_code: string;
    status?: string;
    notes?: string;
    sponsor_name?: string;
    job_title?: string;
  }) => request<Visa>("/api/v1/visas", { method: "POST", body: JSON.stringify(body) }),

  getActiveVisa: () => request<Visa>("/api/v1/visas/active"),

  listTasks: (params?: { status?: string; category?: string; visa_id?: number }) => {
    const sp = new URLSearchParams();
    if (params?.status) sp.set("status", params.status);
    if (params?.category) sp.set("category", params.category);
    if (params?.visa_id) sp.set("visa_id", String(params.visa_id));
    const qs = sp.toString();
    return request<{ tasks: Task[]; count: number }>(
      `/api/v1/tasks${qs ? `?${qs}` : ""}`,
    ).then((r) => r.tasks);
  },

  markTaskDone: (id: number) =>
    request<Task>(`/api/v1/tasks/${id}/done`, { method: "POST" }),

  createLifeEvent: (body: {
    event_type: string;
    occurred_at: string;
    payload?: unknown;
  }) =>
    request<LifeEventResponse>("/api/v1/life-events", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  sendConversationMessage: (messages: ConversationMessage[]) =>
    request<ConversationResponse>("/api/v1/conversation/messages", {
      method: "POST",
      body: JSON.stringify({ messages }),
    }),
};

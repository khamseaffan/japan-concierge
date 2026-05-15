"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import type { Task } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import {
  CheckSquare,
  Circle,
  CheckCircle2,
  ExternalLink,
  MapPin,
  Filter,
} from "lucide-react";

const STATUS_FILTERS = [
  { value: "", label: "All" },
  { value: "pending", label: "Pending" },
  { value: "done", label: "Done" },
] as const;

const SEVERITY_COLORS: Record<string, string> = {
  mandatory: "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
  recommended: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200",
  informational: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
};

const CATEGORY_LABELS: Record<string, string> = {
  pre_arrival: "Pre-arrival",
  immigration: "Immigration",
  municipal: "Municipal",
  tax: "Tax",
  health_insurance: "Health Insurance",
  pension: "Pension",
  banking: "Banking",
  telecom: "Telecom",
  employer: "Employer",
  housing: "Housing",
  general: "General",
};

export default function TasksPage() {
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState("");
  const [expandedId, setExpandedId] = useState<number | null>(null);

  const tasks = useQuery<Task[]>({
    queryKey: ["tasks", { status: statusFilter }],
    queryFn: () =>
      api.listTasks(statusFilter ? { status: statusFilter } : undefined),
  });

  const markDone = useMutation({
    mutationFn: api.markTaskDone,
    onMutate: async (taskId) => {
      await queryClient.cancelQueries({ queryKey: ["tasks"] });
      const previousTasks = queryClient.getQueriesData<Task[]>({
        queryKey: ["tasks"],
      });

      queryClient.setQueriesData<Task[]>({ queryKey: ["tasks"] }, (old) =>
        old?.map((t) =>
          t.id === taskId ? { ...t, status: "done" } : t,
        ),
      );

      return { previousTasks };
    },
    onError: (_err, _taskId, context) => {
      context?.previousTasks.forEach(([key, data]) => {
        queryClient.setQueryData(key, data);
      });
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ["tasks"] });
    },
  });

  const grouped = groupByCategory(tasks.data ?? []);

  return (
    <div className="mx-auto max-w-md px-4 pt-8">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <CheckSquare className="h-6 w-6 text-primary" />
          <h1 className="text-2xl font-bold">Tasks</h1>
        </div>
        {tasks.data && (
          <span className="text-sm text-muted-foreground">
            {tasks.data.filter((t) => t.status !== "done").length} remaining
          </span>
        )}
      </div>

      <div className="flex gap-2 mb-6">
        {STATUS_FILTERS.map((f) => (
          <button
            key={f.value}
            type="button"
            onClick={() => setStatusFilter(f.value)}
            className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 text-sm transition-colors ${
              statusFilter === f.value
                ? "bg-primary text-primary-foreground"
                : "bg-muted text-muted-foreground hover:bg-muted/80"
            }`}
          >
            {f.value === "" && <Filter className="h-3.5 w-3.5" />}
            {f.label}
          </button>
        ))}
      </div>

      {tasks.isLoading && (
        <div className="space-y-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-24 rounded-lg bg-muted animate-pulse" />
          ))}
        </div>
      )}

      {tasks.isError && (
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">
            Failed to load tasks. Is the API running?
          </CardContent>
        </Card>
      )}

      {tasks.data && tasks.data.length === 0 && (
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">
            No tasks yet. Log a life event to generate tasks.
          </CardContent>
        </Card>
      )}

      <div className="space-y-6">
        {Object.entries(grouped).map(([category, categoryTasks]) => (
          <div key={category}>
            <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wide mb-2">
              {CATEGORY_LABELS[category] ?? category}
            </h2>
            <div className="space-y-2">
              {categoryTasks.map((task) => (
                <TaskCard
                  key={task.id}
                  task={task}
                  expanded={expandedId === task.id}
                  onToggle={() =>
                    setExpandedId(expandedId === task.id ? null : task.id)
                  }
                  onMarkDone={() => markDone.mutate(task.id)}
                  isMarking={
                    markDone.isPending && markDone.variables === task.id
                  }
                />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function TaskCard({
  task,
  expanded,
  onToggle,
  onMarkDone,
  isMarking,
}: {
  task: Task;
  expanded: boolean;
  onToggle: () => void;
  onMarkDone: () => void;
  isMarking: boolean;
}) {
  const isDone = task.status === "done";

  return (
    <Card className={isDone ? "opacity-60" : undefined}>
      <CardContent className="py-3 px-4">
        <div className="flex items-start gap-3">
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              if (!isDone) onMarkDone();
            }}
            disabled={isDone || isMarking}
            className="mt-0.5 shrink-0"
          >
            {isDone ? (
              <CheckCircle2 className="h-5 w-5 text-green-600" />
            ) : (
              <Circle className="h-5 w-5 text-muted-foreground hover:text-primary transition-colors" />
            )}
          </button>

          <div className="flex-1 min-w-0" role="button" tabIndex={0} onClick={onToggle} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") onToggle(); }}>
            <div className="flex items-start justify-between gap-2">
              <span
                className={`text-sm font-medium ${isDone ? "line-through" : ""}`}
              >
                {task.title_en}
              </span>
              <Badge
                className={`shrink-0 text-xs ${SEVERITY_COLORS[task.severity] ?? ""}`}
                variant="outline"
              >
                {task.severity}
              </Badge>
            </div>

            {task.deadline_at && (
              <div className="text-xs text-muted-foreground mt-1">
                Due: {task.deadline_at}
              </div>
            )}

            {expanded && (
              <div className="mt-3 space-y-3">
                <Separator />
                <p className="text-sm text-muted-foreground whitespace-pre-line">
                  {task.description_en}
                </p>

                {task.title_ja && (
                  <p className="text-sm text-muted-foreground">
                    {task.title_ja}
                  </p>
                )}

                {task.location_hint && (
                  <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <MapPin className="h-3.5 w-3.5" />
                    {task.location_hint}
                  </div>
                )}

                {task.legal_source_url && (
                  <a
                    href={task.legal_source_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1.5 text-xs text-primary hover:underline"
                  >
                    <ExternalLink className="h-3.5 w-3.5" />
                    Legal source
                  </a>
                )}

                {task.legal_source_text && (
                  <p className="text-xs text-muted-foreground italic">
                    {task.legal_source_text}
                  </p>
                )}

                {!isDone && (
                  <Button
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation();
                      onMarkDone();
                    }}
                    disabled={isMarking}
                  >
                    {isMarking ? "Marking..." : "Mark done"}
                  </Button>
                )}
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function groupByCategory(tasks: Task[]): Record<string, Task[]> {
  const groups: Record<string, Task[]> = {};
  for (const task of tasks) {
    const cat = task.category;
    if (!groups[cat]) groups[cat] = [];
    groups[cat].push(task);
  }
  return groups;
}

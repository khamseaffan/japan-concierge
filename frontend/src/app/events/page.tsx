"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { api, EVENT_TYPE_LABELS } from "@/lib/api";
import type { EventType, LifeEventResponse } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { CalendarPlus, CheckCircle2 } from "lucide-react";

const EVENT_TYPES = Object.keys(EVENT_TYPE_LABELS) as EventType[];

export default function EventsPage() {
  const queryClient = useQueryClient();
  const router = useRouter();
  const [eventType, setEventType] = useState<EventType | null>(null);
  const [occurredAt, setOccurredAt] = useState(
    new Date().toISOString().slice(0, 10),
  );
  const [result, setResult] = useState<LifeEventResponse | null>(null);

  const createEvent = useMutation({
    mutationFn: api.createLifeEvent,
    onSuccess: (data) => {
      setResult(data);
      queryClient.invalidateQueries({ queryKey: ["tasks"] });
    },
  });

  if (result) {
    return (
      <div className="mx-auto max-w-md px-4 pt-8">
        <div className="flex items-center gap-3 mb-6">
          <CheckCircle2 className="h-6 w-6 text-green-600" />
          <h1 className="text-2xl font-bold">Event logged</h1>
        </div>

        <p className="text-muted-foreground mb-4">
          {result.tasks.length} task{result.tasks.length !== 1 && "s"} generated:
        </p>

        <div className="space-y-2 mb-6">
          {result.tasks.map((task) => (
            <Card key={task.id}>
              <CardContent className="py-3 px-4">
                <div className="flex items-start justify-between gap-2">
                  <span className="text-sm font-medium">{task.title_en}</span>
                  <Badge
                    variant={
                      task.severity === "mandatory" ? "default" : "secondary"
                    }
                    className="shrink-0 text-xs"
                  >
                    {task.severity}
                  </Badge>
                </div>
                {task.deadline_at && (
                  <div className="text-xs text-muted-foreground mt-1">
                    Due: {task.deadline_at}
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>

        <div className="flex gap-3">
          <Button
            variant="outline"
            className="flex-1"
            onClick={() => {
              setResult(null);
              setEventType(null);
            }}
          >
            Log another
          </Button>
          <Button className="flex-1" onClick={() => router.push("/tasks")}>
            View tasks
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-md px-4 pt-8">
      <div className="flex items-center gap-3 mb-6">
        <CalendarPlus className="h-6 w-6 text-primary" />
        <h1 className="text-2xl font-bold">Log Life Event</h1>
      </div>

      <div className="space-y-3 mb-6">
        <Label className="text-base font-medium">What happened?</Label>
        {EVENT_TYPES.map((et) => (
          <button
            key={et}
            type="button"
            onClick={() => setEventType(et)}
            className={`w-full text-left rounded-lg border p-4 transition-colors ${
              eventType === et
                ? "border-primary bg-primary/5"
                : "border-border hover:border-foreground/20"
            }`}
          >
            <div className="font-medium">{EVENT_TYPE_LABELS[et]}</div>
          </button>
        ))}
      </div>

      <div className="space-y-2 mb-6">
        <Label htmlFor="date">When did it happen?</Label>
        <Input
          id="date"
          type="date"
          value={occurredAt}
          onChange={(e) => setOccurredAt(e.target.value)}
        />
      </div>

      <Button
        className="w-full"
        size="lg"
        disabled={!eventType || !occurredAt || createEvent.isPending}
        onClick={() => {
          if (!eventType || !occurredAt) return;
          createEvent.mutate({
            event_type: eventType,
            occurred_at: occurredAt,
          });
        }}
      >
        {createEvent.isPending ? "Logging..." : "Log event"}
      </Button>

      {createEvent.isError && (
        <p className="text-destructive text-sm mt-2 text-center">
          {createEvent.error.message}
        </p>
      )}
    </div>
  );
}

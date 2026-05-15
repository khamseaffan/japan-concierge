"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { api } from "@/lib/api";
import type { Visa, VisaType } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Plane } from "lucide-react";

export default function VisaPage() {
  const queryClient = useQueryClient();
  const router = useRouter();

  const activeVisa = useQuery<Visa>({
    queryKey: ["visa", "active"],
    queryFn: api.getActiveVisa,
    retry: false,
  });

  const visaTypes = useQuery<VisaType[]>({
    queryKey: ["visaTypes"],
    queryFn: api.listVisaTypes,
    enabled: activeVisa.isError,
  });

  if (activeVisa.isLoading) {
    return <LoadingScreen />;
  }

  if (activeVisa.data) {
    return <ActiveVisaView visa={activeVisa.data} />;
  }

  return (
    <OnboardingFlow
      visaTypes={visaTypes.data ?? []}
      isLoading={visaTypes.isLoading}
      onCreated={(visa) => {
        queryClient.setQueryData(["visa", "active"], visa);
        router.push("/events");
      }}
    />
  );
}

function LoadingScreen() {
  return (
    <div className="flex items-center justify-center min-h-[60vh]">
      <div className="animate-pulse text-muted-foreground">Loading...</div>
    </div>
  );
}

function ActiveVisaView({ visa }: { visa: Visa }) {
  return (
    <div className="mx-auto max-w-md px-4 pt-8">
      <div className="flex items-center gap-3 mb-6">
        <Plane className="h-6 w-6 text-primary" />
        <h1 className="text-2xl font-bold">Your Visa</h1>
      </div>
      <Card>
        <CardHeader>
          <CardTitle className="uppercase tracking-wide">
            {visa.visa_type_code}
          </CardTitle>
          <CardDescription>
            Status: <span className="capitalize">{visa.status}</span>
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          {visa.sponsor_name && (
            <div>
              <span className="text-muted-foreground">Sponsor:</span>{" "}
              {visa.sponsor_name}
            </div>
          )}
          {visa.job_title && (
            <div>
              <span className="text-muted-foreground">Job title:</span>{" "}
              {visa.job_title}
            </div>
          )}
          {visa.expires_at && (
            <div>
              <span className="text-muted-foreground">Expires:</span>{" "}
              {visa.expires_at}
            </div>
          )}
          <div className="text-xs text-muted-foreground pt-2">
            Created {new Date(visa.created_at).toLocaleDateString()}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function OnboardingFlow({
  visaTypes,
  isLoading,
  onCreated,
}: {
  visaTypes: VisaType[];
  isLoading: boolean;
  onCreated: (visa: Visa) => void;
}) {
  const [selected, setSelected] = useState<string | null>(null);
  const [sponsorName, setSponsorName] = useState("");
  const [jobTitle, setJobTitle] = useState("");

  const createVisa = useMutation({
    mutationFn: api.createVisa,
    onSuccess: onCreated,
  });

  return (
    <div className="mx-auto max-w-md px-4 pt-8">
      <div className="mb-8 text-center">
        <Plane className="mx-auto h-10 w-10 text-primary mb-3" />
        <h1 className="text-2xl font-bold">Japan Concierge</h1>
        <p className="text-muted-foreground mt-1">
          Track your immigration compliance
        </p>
      </div>

      <div className="space-y-3 mb-6">
        <Label className="text-base font-medium">Select your visa type</Label>
        {isLoading ? (
          <div className="space-y-2">
            {[1, 2].map((i) => (
              <div
                key={i}
                className="h-20 rounded-lg bg-muted animate-pulse"
              />
            ))}
          </div>
        ) : (
          visaTypes.map((vt) => (
            <button
              key={vt.code}
              type="button"
              onClick={() => setSelected(vt.code)}
              className={`w-full text-left rounded-lg border p-4 transition-colors ${
                selected === vt.code
                  ? "border-primary bg-primary/5"
                  : "border-border hover:border-foreground/20"
              }`}
            >
              <div className="font-medium">{vt.display_name_en}</div>
              <div className="text-sm text-muted-foreground">
                {vt.display_name_ja}
              </div>
            </button>
          ))
        )}
      </div>

      {selected && (
        <div className="space-y-4 mb-6">
          <div className="space-y-2">
            <Label htmlFor="sponsor">Sponsor name (optional)</Label>
            <Input
              id="sponsor"
              placeholder="e.g. company name"
              value={sponsorName}
              onChange={(e) => setSponsorName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="job">Job title (optional)</Label>
            <Input
              id="job"
              placeholder="e.g. Software Engineer"
              value={jobTitle}
              onChange={(e) => setJobTitle(e.target.value)}
            />
          </div>
        </div>
      )}

      <Button
        className="w-full"
        size="lg"
        disabled={!selected || createVisa.isPending}
        onClick={() => {
          if (!selected) return;
          createVisa.mutate({
            visa_type_code: selected,
            ...(sponsorName && { sponsor_name: sponsorName }),
            ...(jobTitle && { job_title: jobTitle }),
          });
        }}
      >
        {createVisa.isPending ? "Creating..." : "Get started"}
      </Button>

      {createVisa.isError && (
        <p className="text-destructive text-sm mt-2 text-center">
          {createVisa.error.message}
        </p>
      )}
    </div>
  );
}

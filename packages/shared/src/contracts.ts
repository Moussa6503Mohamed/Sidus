export type SyllabusCode = "0610" | "5090";

export interface CanonicalExplanationKey {
  question: string;
  syllabus: SyllabusCode;
  rubric: string;
  language: string;
  explanationVersion: string;
}

export interface HealthResponse { service: string; status: "ok"; }

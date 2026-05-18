import { useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  ArrowRight,
  BarChart3,
  BrainCircuit,
  CheckCircle2,
  FileText,
  Target,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { APP_BRAND_NAME } from "@/lib/branding";
import { ROUTES } from "@/lib/constants";
import {
  MARKETING_ADVANTAGES,
  MARKETING_OUTCOMES,
  MARKETING_TEXT,
  MARKETING_WORKFLOW,
  selectMarketingHeroVideoSrc,
} from "@/pages/marketing/marketingHome.constants";

const ADVANTAGE_ICON_MAP: Record<string, LucideIcon> = {
  target: Target,
  brain: BrainCircuit,
  resume: FileText,
  report: BarChart3,
};

export default function MarketingHomePage() {
  const navigate = useNavigate();
  const [heroVideoSrc] = useState(selectMarketingHeroVideoSrc);

  const handleStartNow = useCallback(() => {
    navigate(ROUTES.auth, {
      state: {
        forceAuth: true,
        from: {
          pathname: ROUTES.interviewIntro,
        },
      },
    });
  }, [navigate]);

  return (
    <div className="h-full w-full overflow-y-auto bg-slate-50">
      <section className="relative h-screen min-h-[720px] overflow-hidden text-white">
        <video
          className="absolute inset-0 h-full w-full object-cover"
          autoPlay
          muted
          loop
          playsInline
          preload="metadata"
          aria-hidden="true"
        >
          <source src={heroVideoSrc} type="video/mp4" />
        </video>
        <div
          className="absolute inset-0 bg-gradient-to-r from-slate-900/70 via-slate-900/45 to-slate-900/25"
          aria-hidden="true"
        />

        <header className="absolute inset-x-0 top-0 z-20 bg-transparent">
          <div className="flex h-16 w-full items-center justify-between px-6 md:px-8 lg:px-10">
            <div className="flex items-center gap-3">
              <img
                src="/prism-logo.png"
                alt={APP_BRAND_NAME}
                className="h-8 w-8 rounded-lg border border-white/25 object-cover"
              />
              <span className="text-base font-semibold">{APP_BRAND_NAME}</span>
            </div>
            <Button
              variant="outline"
              className="rounded-full border-white/35 bg-white/10 text-white hover:bg-white/20 hover:text-white"
              onClick={handleStartNow}
            >
              {MARKETING_TEXT.startNow}
            </Button>
          </div>
        </header>

        <div className="relative z-10 mx-auto flex h-full min-h-[720px] w-full max-w-6xl items-end px-6 pt-24 pb-[12vh] md:px-8 md:pb-[14vh]">
          <div className="max-w-2xl space-y-6">
            <h1 className="hero-slide-in hero-delay-1 text-5xl font-semibold leading-tight md:text-7xl">
              {MARKETING_TEXT.heroTitle}
            </h1>
            <p className="hero-slide-in hero-delay-2 text-xl leading-relaxed text-slate-100/95 md:text-2xl">
              {MARKETING_TEXT.heroSubtitle}
            </p>
            <Button
              className="hero-slide-in hero-delay-3 h-11 rounded-full px-7 text-base"
              onClick={handleStartNow}
            >
              {MARKETING_TEXT.startNow}
              <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
          </div>
        </div>
      </section>

      <main className="mx-auto w-full max-w-6xl space-y-8 px-6 py-10 md:px-8 md:py-12">
        <section className="space-y-5 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm md:p-8">
          <h2 className="text-3xl font-semibold text-slate-900">
            {MARKETING_TEXT.advantagesTitle}
          </h2>
          <div className="grid gap-4 md:grid-cols-2">
            {MARKETING_ADVANTAGES.map((item) => {
              const Icon = ADVANTAGE_ICON_MAP[item.icon] ?? Target;
              return (
                <article
                  key={item.title}
                  className="rounded-xl border border-slate-200 bg-slate-50 p-5"
                >
                  <div className="mb-3 inline-flex rounded-lg bg-cyan-50 p-2 text-cyan-700">
                    <Icon className="h-5 w-5" />
                  </div>
                  <h3 className="text-lg font-semibold text-slate-900">
                    {item.title}
                  </h3>
                  <p className="mt-1.5 text-slate-600">{item.description}</p>
                </article>
              );
            })}
          </div>
        </section>

        <section className="grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
          <article className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm md:p-8">
            <h2 className="text-2xl font-semibold text-slate-900">
              {MARKETING_TEXT.workflowTitle}
            </h2>
            <ol className="mt-5 space-y-3">
              {MARKETING_WORKFLOW.map((step, index) => (
                <li
                  key={step}
                  className="flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50 px-4 py-3"
                >
                  <span className="mt-0.5 inline-flex h-6 w-6 items-center justify-center rounded-full bg-cyan-100 text-xs font-semibold text-cyan-700">
                    {index + 1}
                  </span>
                  <span className="text-slate-700">{step}</span>
                </li>
              ))}
            </ol>
          </article>

          <article className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm md:p-8">
            <h2 className="text-2xl font-semibold text-slate-900">
              {MARKETING_TEXT.outcomeTitle}
            </h2>
            <ul className="mt-5 space-y-3">
              {MARKETING_OUTCOMES.map((outcome) => (
                <li
                  key={outcome}
                  className="flex items-start gap-3 text-slate-700"
                >
                  <CheckCircle2 className="mt-0.5 h-5 w-5 text-cyan-700" />
                  {outcome}
                </li>
              ))}
            </ul>
          </article>
        </section>
      </main>
    </div>
  );
}

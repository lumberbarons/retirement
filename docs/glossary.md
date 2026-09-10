# Glossary

Expansions of the acronyms used across the specs and ADRs. Not all of these are
in scope for the first epic; some appear only as out-of-scope notes or as
sources the engine relies on.

| Term | Meaning |
|---|---|
| ACB | Adjusted Cost Base — the tax cost of a holding in a non-registered account; capital gains are proceeds minus ACB. |
| ADR | Architectural Decision Record — a standalone record of a decision and its rationale; lives in `docs/adr/`. |
| AFNI | Adjusted Family Net Income — the income measure used to phase out some Ontario credits and benefits. |
| ALDA | Advanced Life Deferred Annuity — an annuity purchased now that begins paying at a later age; a longevity tool for a zero-bequest plan. |
| BPA | Basic Personal Amount — the non-refundable federal credit that everyone claims; phases out at high income. |
| CANSIM | Canadian Socio-economic Information Management System — Statistics Canada's data series, used for the LIF maximum factor. |
| CI | Continuous Integration — the automated build/test pipeline that runs the ±$1 validation suite on every commit. |
| CLI | Command Line Interface — the `retire` command-line tool that is this epic's primary deliverable. |
| CPP | Canada Pension Plan — the earnings-related public pension. |
| CPP2 | The second, enhanced CPP contribution tier on earnings between YMPE and YAMPE. |
| CPI | Consumer Price Index — the inflation measure that indexes tax brackets, credits, the TFSA limit, OAS, and CPP-in-pay. |
| CPM2014 | Canadian Pensioners' Mortality (2014) — the mortality table behind the probabilistic longevity model. |
| CRA | Canada Revenue Agency — the primary source for brackets, credits, and forms. |
| CSV | Comma-Separated Values — the machine-readable report format the CLI emits. |
| DB | Defined Benefit — a pension whose payout is set by formula rather than by investment returns. |
| DTC | Dividend Tax Credit — the credit (federal and Ontario) on grossed-up dividend income. |
| ESDC | Employment and Social Development Canada — the primary source for CPP/OAS/GIS rate cards. |
| FHSA | First Home Savings Account — an account type noted as out of scope for this household. |
| FMV | Fair Market Value — the value used for deemed disposition of non-registered assets at death. |
| FP Canada | The national financial-planning body that publishes the Projection Assumption Guidelines used as default returns. |
| FSRA | Financial Services Regulatory Authority of Ontario — publishes the LIF F-factor and governs unlocking. |
| FTC | Foreign Tax Credit — the credit claimed against Canadian tax on foreign-sourced income. |
| GIS | Guaranteed Income Supplement — the income-tested top-up to OAS for low-income seniors. |
| GST/HST | Goods and Services Tax / Harmonized Sales Tax — the consumption-tax credit that can reappear for a low-income survivor. |
| HTTP | HyperText Transfer Protocol — the transport a future web frontend would use to reach the engine. |
| IO | Input/Output — terminal, file, and network access, which this project confines to the CLI and report packages. |
| ISP1002 | Service Canada form used to elect CPP retirement pension sharing between spouses. |
| ITR | Income Tax Regulations — the regulations whose 7308 schedule sets the RRIF minimum factors. |
| LIF | Life Income Fund — the locked-in successor to a LIRA, with both a minimum and a maximum withdrawal. |
| LIRA | Locked-In Retirement Account — where a commuted pension value lands before conversion to a LIF. |
| LTC | Long-Term Care — the late-life care spending modelled as its own stream. |
| NOEC | Northern Ontario Energy Credit — a component of the Ontario Trillium Benefit. |
| OAS | Old Age Security — the residence-based public pension, subject to a 15% recovery tax. |
| OEPTC | Ontario Energy and Property Tax Credit — a component of the Ontario Trillium Benefit. |
| OHP | Ontario Health Premium — the Ontario surcharge on taxable income, added on top of income tax. |
| OSHPTG | Ontario Senior Homeowners' Property Tax Grant — an income-tested credit for seniors aged 64+. |
| OSTC | Ontario Sales Tax Credit — a component of the Ontario Trillium Benefit. |
| OTB | Ontario Trillium Benefit — the combined OEPTC + OSTC + NOEC payment. |
| PA | Pension Adjustment — the value that reduces RRSP room for an active DB plan member. |
| PBA | Pension Benefits Act (Ontario) — governs DB survivor options and locked-in rules. |
| PRB | Post-Retirement Benefit — the CPP top-up for contributors who work while receiving CPP. |
| RDSP | Registered Disability Savings Plan — an account type noted as out of scope. |
| RESP | Registered Education Savings Plan — out of scope for a household with no children. |
| RPP | Registered Pension Plan — the employer plan behind DB pension income. |
| RRIF | Registered Retirement Income Fund — the RRSP's decumulation form, with age-based minimums. |
| RRSP | Registered Retirement Savings Plan — the tax-deferred accumulation account. |
| T1032 | CRA form for joint election to split eligible pension income (pension income splitting). |
| T4032-ON | CRA Payroll Deductions Tables for Ontario — the source cited for Ontario personal tax credits. |
| TFSA | Tax-Free Savings Account — withdrawals are tax-free and do not enter net income. |
| UI | User Interface — the graphical front end deferred to a later epic. |
| WASM | WebAssembly — a compile target that would be needed to run the engine in-browser. |
| YAMPE | Year's Additional Maximum Pensionable Earnings — the top of the CPP2 earnings band. |
| YBE | Year's Basic Exemption — the earnings floor below which no CPP contributions apply. |
| YAML | A human-readable data-serialization format; the household config file is YAML. |
| YMPE | Year's Maximum Pensionable Earnings — the ceiling of the base CPP earnings band. |

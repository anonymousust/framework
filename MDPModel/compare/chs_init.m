clear

global alphaPower;
global k;

% k=5/10/20/30/40

alphaPower = 0.3;
k = 5;


disp(['alpha=',num2str(alphaPower)])

% baseline_SolveStrategy_latency; 

naive_SolveStrategy_latency;

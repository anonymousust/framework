global numOfStates; 
% cs = 0,1,2,3
% la = 0,1
% lh = 0,1,2
% leader A/H = 0/1
numOfStates = 48;
global alphaPower;

% actions: 1 follow, 2 silent
choices = 2;
follow = 1; silent = 2;

global k;
delta=1;
Delta = delta*k;

global rou latency;
global P T Rc;

%%% transition
P = cell(1,choices);
T = cell(1,choices);
Rc = cell(1,choices);
latency = cell(1,choices);
for i = 1:choices
    P{i} = sparse(numOfStates, numOfStates);
    T{i} = sparse(numOfStates, numOfStates);
    Rc{i} = sparse(numOfStates, numOfStates);
    latency{i} = sparse(numOfStates, numOfStates);
end

H_H_time = 3*delta;
H_A_time = delta+2*Delta;
A_H_time = delta+2*Delta;
A_A_time = 3*Delta;
silent_A_A_time = 2*Delta;
silent_A_H_time = Delta+delta;

for state = 1:numOfStates
    [cs,prev_leader,lh,leader] = chs_stnum2st(state);
    % next_cs denote result of cs+1
    if cs < 3
        next_cs = cs+1;
    elseif cs==3
        next_cs = 3;
    end
    
    % define follow
    if leader == 0 % A
        % for complete
        P{follow}(state, 1) = 1;
        Rc{follow}(state, 1) = 10000;
        T{follow}(state, 1) = 1;
    else % H
        if lh < 2
            lh=lh+1;
        end
        P{follow}(state, chs_st2stnum(next_cs,1,lh,0)) = alphaPower;
        P{follow}(state, chs_st2stnum(next_cs,1,lh,1)) = 1-alphaPower;
        T{follow}(state, chs_st2stnum(next_cs,1,lh,0)) = H_A_time;
        T{follow}(state, chs_st2stnum(next_cs,1,lh,1)) = H_H_time;
        if cs == 3
            Rc{follow}(state, chs_st2stnum(next_cs,1,lh,0)) = 1;
            Rc{follow}(state, chs_st2stnum(next_cs,1,lh,1)) = 1;
        end
    end
    
    % define silent
    if leader == 0 % A
        if prev_leader == 1 && lh>0
            lh=lh-1;
        end
        P{silent}(state, chs_st2stnum(0,0,lh,0)) = alphaPower;
        P{silent}(state, chs_st2stnum(0,0,lh,1)) = 1-alphaPower;
        T{silent}(state, chs_st2stnum(0,0,lh,0)) = silent_A_A_time;
        T{silent}(state, chs_st2stnum(0,0,lh,1)) = silent_A_H_time;
    else % H
        % for complete
        P{silent}(state, 1) = 1;
        Rc{silent}(state, 1) = 10000;
        T{silent}(state, 1) = 1;
    end
    
end

disp(mdp_check(P, Rc))

epsilon = 0.0001;

lowRou = 0;
highRou = 1;
while(highRou - lowRou > epsilon/8)
    rou = (highRou + lowRou) / 2;
    for i = 1:choices
        latency{i} = (1-rou).*T{i} - Rc{i};
    end
    [latencyPolicy, reward, cpuTime] = mdp_relative_value_iteration(P, latency, epsilon/8);
    if(reward > 0)
        lowRou = rou;
    else
        highRou = rou;
    end
end
disp('Latency: ')
format long
disp(1-rou)

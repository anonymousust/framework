global numOfStates; 
% cs = 0,1,2,3
% la = 0,1
% lh = 0
% leader A/H = 0/1
numOfStates = 16;
global alphaPower;

% actions: 1 adopt, 2 wait, 3 release, 4 withhold, 5 silent
choices = 5;
adopt = 1; wait = 2; release = 3; withhold = 4; silent = 5;

global k;
delta=1;
Delta = delta*k;
time = 2*Delta;

global rou latency;
global P T Rc;

%%% transition
P = cell(1,choices);
T = cell(1,choices);
Rc = cell(1,choices);
latency = cell(1,choices);
for i = 1:choices
    %%% sparse 全零稀疏矩阵
    P{i} = sparse(numOfStates, numOfStates);
    T{i} = sparse(numOfStates, numOfStates);
    Rc{i} = sparse(numOfStates, numOfStates);
    latency{i} = sparse(numOfStates, numOfStates);
end

for state = 1:numOfStates
    [cs,la,lh,leader] = streamlet_stnum2st(state);
    % next_cs denote result of cs+1
    if cs < 3
        next_cs = cs+1;
    else % cs==3
        next_cs = 3;
    end
    
    % define adopt
    if leader == 0 % adopt-A
        if la == 0
            cs_adopt_a = cs;
        else
            cs_adopt_a = 0;
        end
        P{adopt}(state, streamlet_st2stnum(cs_adopt_a,1,0,0)) = alphaPower;
        P{adopt}(state, streamlet_st2stnum(cs_adopt_a,1,0,1)) = 1-alphaPower;
        T{adopt}(state, streamlet_st2stnum(cs_adopt_a,1,0,0)) = time;
        T{adopt}(state, streamlet_st2stnum(cs_adopt_a,1,0,1)) = time;
    else % adopt-H
        if la == 0
            cs_adopt_h = next_cs;
        else
            cs_adopt_h = 1;
        end
        P{adopt}(state, streamlet_st2stnum(cs_adopt_h,0,0,0)) = alphaPower;
        P{adopt}(state, streamlet_st2stnum(cs_adopt_h,0,0,1)) = 1-alphaPower;
        T{adopt}(state, streamlet_st2stnum(cs_adopt_h,0,0,0)) = time;
        T{adopt}(state, streamlet_st2stnum(cs_adopt_h,0,0,1)) = time;
        if cs_adopt_h == 3
            Rc{adopt}(state, streamlet_st2stnum(cs_adopt_h,0,0,0)) = 1;
            Rc{adopt}(state, streamlet_st2stnum(cs_adopt_h,0,0,1)) = 1;
        end
    end
    
    
    % define wait
    if leader == 0 % wait-A
        if la == 0
            P{wait}(state, streamlet_st2stnum(0,0,0,0)) = alphaPower;
            P{wait}(state, streamlet_st2stnum(0,0,0,1)) = 1-alphaPower;
            T{wait}(state, streamlet_st2stnum(0,0,0,0)) = time;
            T{wait}(state, streamlet_st2stnum(0,0,0,1)) = time;
        else % la == 1
            P{wait}(state, streamlet_st2stnum(next_cs,1,0,0)) = alphaPower;
            P{wait}(state, streamlet_st2stnum(next_cs,1,0,1)) = 1-alphaPower;
            T{wait}(state, streamlet_st2stnum(next_cs,1,0,0)) = time;
            T{wait}(state, streamlet_st2stnum(next_cs,1,0,1)) = time;
            if next_cs == 3
                Rc{wait}(state, streamlet_st2stnum(next_cs,1,0,0)) = 1;
                Rc{wait}(state, streamlet_st2stnum(next_cs,1,0,1)) = 1;
            end
        end
    else % wait - H
        if la == 0
            cs_wait_h = next_cs;
        else % la == 1
            cs_wait_h = 1;
        end
        P{wait}(state, streamlet_st2stnum(cs_wait_h,0,0,0)) = alphaPower;
        P{wait}(state, streamlet_st2stnum(cs_wait_h,0,0,1)) = 1-alphaPower;
        T{wait}(state, streamlet_st2stnum(cs_wait_h,0,0,0)) = time;
        T{wait}(state, streamlet_st2stnum(cs_wait_h,0,0,1)) = time;
        if cs_wait_h == 3
            Rc{wait}(state, streamlet_st2stnum(cs_wait_h,0,0,0)) = 1;
            Rc{wait}(state, streamlet_st2stnum(cs_wait_h,0,0,1)) = 1;
        end
    end
    
    % define release
    if la == 1
        if leader == 0 % release-A
            P{release}(state, streamlet_st2stnum(next_cs,1,0,0)) = alphaPower;
            P{release}(state, streamlet_st2stnum(next_cs,1,0,1)) = 1-alphaPower;
            T{release}(state, streamlet_st2stnum(next_cs,1,0,0)) = time;
            T{release}(state, streamlet_st2stnum(next_cs,1,0,1)) = time;
            if next_cs == 3
                Rc{release}(state, streamlet_st2stnum(next_cs,1,0,0)) = 1;
                Rc{release}(state, streamlet_st2stnum(next_cs,1,0,1)) = 1;
            end
        else % release-H
            if cs < 2 % 0 or 1
                cs_release_h = cs+2;
            else
                cs_release_h = 3;
            end
            P{release}(state, streamlet_st2stnum(cs_release_h,0,0,0)) = alphaPower;
            P{release}(state, streamlet_st2stnum(cs_release_h,0,0,1)) = 1-alphaPower;
            T{release}(state, streamlet_st2stnum(cs_release_h,0,0,0)) = time;
            T{release}(state, streamlet_st2stnum(cs_release_h,0,0,1)) = time;
            if cs == 3
                Rc{release}(state, streamlet_st2stnum(cs_release_h,0,0,0)) = 2;
                Rc{release}(state, streamlet_st2stnum(cs_release_h,0,0,1)) = 2;
            else
                Rc{release}(state, streamlet_st2stnum(cs_release_h,0,0,0)) = cs;
                Rc{release}(state, streamlet_st2stnum(cs_release_h,0,0,1)) = cs;
            end
        end
    else
        % for completeness
        P{release}(state, 1) = 1;
        Rc{release}(state, 1) = 10000;
        T{release}(state, 1) = time;
    end
    
    % define withhold
    if la == 1
        if leader == 0 % adversary
            P{withhold}(state, streamlet_st2stnum(next_cs,1,0,0)) = alphaPower;
            P{withhold}(state, streamlet_st2stnum(next_cs,1,0,1)) = 1-alphaPower;
            T{withhold}(state, streamlet_st2stnum(next_cs,1,0,0)) = time;
            T{withhold}(state, streamlet_st2stnum(next_cs,1,0,1)) = time;
            if next_cs == 3
                Rc{withhold}(state, streamlet_st2stnum(next_cs,1,0,0)) = 1;
                Rc{withhold}(state, streamlet_st2stnum(next_cs,1,0,1)) = 1;
            end
        else % honest
            P{withhold}(state, streamlet_st2stnum(0,0,0,0)) = alphaPower;
            P{withhold}(state, streamlet_st2stnum(0,0,0,1)) = 1-alphaPower;
            T{withhold}(state, streamlet_st2stnum(0,0,0,0)) = time;
            T{withhold}(state, streamlet_st2stnum(0,0,0,1)) = time;
            if cs == 2 || cs == 3
                Rc{withhold}(state, streamlet_st2stnum(0,0,0,0)) = 1;
                Rc{withhold}(state, streamlet_st2stnum(0,0,0,1)) = 1;
            end
        end
    else
        P{withhold}(state, 1) = 1;
        Rc{withhold}(state, 1) = 10000;
        T{withhold}(state, 1) = time;
    end
    
    % define silent
    if leader == 0 % silent-A
        P{silent}(state, streamlet_st2stnum(0,0,0,0)) = alphaPower;
        P{silent}(state, streamlet_st2stnum(0,0,0,1)) = 1-alphaPower;
        T{silent}(state, streamlet_st2stnum(0,0,0,0)) = time;
        T{silent}(state, streamlet_st2stnum(0,0,0,1)) = time;
    else % silent - H
        if la == 0
            cs_silent_h = next_cs;
        else % la == 1
            cs_silent_h = 1;
        end
        P{silent}(state, streamlet_st2stnum(cs_silent_h,0,0,0)) = alphaPower;
        P{silent}(state, streamlet_st2stnum(cs_silent_h,0,0,1)) = 1-alphaPower;
        T{silent}(state, streamlet_st2stnum(cs_silent_h,0,0,0)) = time;
        T{silent}(state, streamlet_st2stnum(cs_silent_h,0,0,1)) = time;
        if cs_silent_h == 3
            Rc{silent}(state, streamlet_st2stnum(cs_silent_h,0,0,0)) = 1;
            Rc{silent}(state, streamlet_st2stnum(cs_silent_h,0,0,1)) = 1;
        end
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

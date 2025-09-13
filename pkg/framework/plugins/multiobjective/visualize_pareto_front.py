#!/usr/bin/env python3
"""
Multi-Objective Kubernetes Descheduler - Pareto Front Visualization

This script creates 3D and 2D visualizations of the Pareto front evolution
from the multi-objective optimization integration tests.

Usage:
    python visualize_pareto_front.py optimization_results_*.json
"""

import json
import numpy as np
import matplotlib.pyplot as plt
from mpl_toolkits.mplot3d import Axes3D
import seaborn as sns
import pandas as pd
import sys
import os
from pathlib import Path

# Set style
plt.style.use('seaborn-v0_8')
sns.set_palette("husl")

def load_optimization_data(filename):
    """Load optimization results from JSON file."""
    with open(filename, 'r') as f:
        return json.load(f)

def calculate_hypervolume(pareto_front, reference_point=None):
    """
    Calculate hypervolume indicator for a Pareto front.
    
    Args:
        pareto_front: List of solutions with cost, disruption, balance
        reference_point: Reference point for hypervolume (worst case scenario)
    
    Returns:
        Hypervolume value (higher is better)
    """
    if not pareto_front:
        return 0.0
    
    # Extract objectives (cost, disruption, balance - all minimization)
    objectives = np.array([[sol['cost'], sol['disruption'], sol['balance']] 
                          for sol in pareto_front])
    
    if reference_point is None:
        # Use worst point + small margin as reference
        reference_point = np.max(objectives, axis=0) + 0.1
    
    # Simple hypervolume calculation for 3D case
    # For production use, consider using pymoo or deap for exact calculation
    hypervolume = 0.0
    
    # Monte Carlo approximation for hypervolume
    n_samples = 10000
    np.random.seed(42)  # For reproducible results
    
    # Generate random points in the hypervolume space
    min_point = np.min(objectives, axis=0)
    random_points = np.random.uniform(min_point, reference_point, (n_samples, 3))
    
    # Count points dominated by at least one solution in Pareto front
    dominated_count = 0
    for point in random_points:
        # Check if point is dominated by any Pareto front solution
        dominated = False
        for obj in objectives:
            if np.all(obj <= point):  # obj dominates point (minimization)
                dominated = True
                break
        if dominated:
            dominated_count += 1
    
    # Hypervolume = (dominated_fraction) * (total_volume)
    total_volume = np.prod(reference_point - min_point)
    hypervolume = (dominated_count / n_samples) * total_volume
    
    return hypervolume

def calculate_sparsity(pareto_front):
    """
    Calculate sparsity (diversity) of solutions in Pareto front.
    Uses average crowding distance as sparsity measure.
    
    Args:
        pareto_front: List of solutions with cost, disruption, balance
    
    Returns:
        Average crowding distance (higher is better - more diverse)
    """
    if len(pareto_front) <= 2:
        return float('inf')  # Very sparse for small fronts
    
    # Extract objectives
    objectives = np.array([[sol['cost'], sol['disruption'], sol['balance']] 
                          for sol in pareto_front])
    
    n_solutions = len(objectives)
    n_objectives = objectives.shape[1]
    
    # Calculate crowding distance for each solution
    crowding_distances = np.zeros(n_solutions)
    
    for m in range(n_objectives):  # For each objective
        # Sort by objective m
        sorted_indices = np.argsort(objectives[:, m])
        
        # Boundary solutions get infinite distance
        crowding_distances[sorted_indices[0]] = float('inf')
        crowding_distances[sorted_indices[-1]] = float('inf')
        
        # Calculate distances for intermediate solutions
        obj_range = objectives[sorted_indices[-1], m] - objectives[sorted_indices[0], m]
        if obj_range > 0:  # Avoid division by zero
            for i in range(1, n_solutions - 1):
                distance = (objectives[sorted_indices[i+1], m] - 
                           objectives[sorted_indices[i-1], m]) / obj_range
                crowding_distances[sorted_indices[i]] += distance
    
    # Return average crowding distance (excluding infinite values)
    finite_distances = crowding_distances[np.isfinite(crowding_distances)]
    if len(finite_distances) > 0:
        return np.mean(finite_distances)
    else:
        return float('inf')

def calculate_metrics_over_time(data):
    """Calculate hypervolume and sparsity metrics for each round."""
    metrics = {
        'rounds': [],
        'hypervolume': [],
        'sparsity': [],
        'pareto_size': []
    }
    
    # Find global reference point for consistent hypervolume calculation
    all_objectives = []
    for round_data in data['rounds']:
        for sol in round_data['paretoFront']:
            all_objectives.append([sol['cost'], sol['disruption'], sol['balance']])
    
    if all_objectives:
        all_objectives = np.array(all_objectives)
        reference_point = np.max(all_objectives, axis=0) + 0.1
    else:
        reference_point = np.array([1.1, 1.1, 1.1])  # Default reference
    
    # Calculate metrics for each round
    for i, round_data in enumerate(data['rounds']):
        pareto_front = round_data['paretoFront']
        
        hypervolume = calculate_hypervolume(pareto_front, reference_point)
        sparsity = calculate_sparsity(pareto_front)
        
        metrics['rounds'].append(i + 1)
        metrics['hypervolume'].append(hypervolume)
        metrics['sparsity'].append(sparsity if sparsity != float('inf') else 100)  # Cap infinite values
        metrics['pareto_size'].append(len(pareto_front))
    
    return metrics

def create_3d_pareto_plot(data, round_num=None):
    """Create 3D scatter plot of Pareto front."""
    fig = plt.figure(figsize=(12, 9))
    ax = fig.add_subplot(111, projection='3d')
    
    if round_num is not None:
        # Plot specific round
        round_data = data['rounds'][round_num - 1]
        solutions = round_data['paretoFront']
        
        costs = [sol['cost'] for sol in solutions]
        disruptions = [sol['disruption'] for sol in solutions]
        balances = [sol['balance'] for sol in solutions]
        
        scatter = ax.scatter(costs, disruptions, balances, 
                           c=range(len(solutions)), cmap='viridis', s=60, alpha=0.7)
        
        # Highlight best solution
        best = round_data['bestSolution']
        ax.scatter([best['cost']], [best['disruption']], [best['balance']], 
                  c='red', s=200, marker='*', label='Best Solution', alpha=0.9)
        
        ax.set_title(f'{data["testCase"]["name"]} - Round {round_num}\n'
                    f'Pareto Front ({len(solutions)} solutions)', fontsize=14, pad=20)
    else:
        # Plot evolution across all rounds
        colors = plt.cm.plasma(np.linspace(0, 1, len(data['rounds'])))
        
        for i, round_data in enumerate(data['rounds']):
            solutions = round_data['paretoFront']
            costs = [sol['cost'] for sol in solutions]
            disruptions = [sol['disruption'] for sol in solutions]
            balances = [sol['balance'] for sol in solutions]
            
            ax.scatter(costs, disruptions, balances, 
                      c=[colors[i]], s=40, alpha=0.6, label=f'Round {i+1}')
        
        ax.set_title(f'{data["testCase"]["name"]} - Evolution Across Rounds\n'
                    f'Pareto Front Evolution', fontsize=14, pad=20)
        ax.legend(bbox_to_anchor=(1.05, 1), loc='upper left')
    
    ax.set_xlabel('Cost (normalized)', fontsize=12)
    ax.set_ylabel('Disruption (normalized)', fontsize=12)
    ax.set_zlabel('Balance (normalized)', fontsize=12)
    
    # Add colorbar for single round
    if round_num is not None:
        plt.colorbar(scatter, ax=ax, shrink=0.5, aspect=5, label='Solution ID')
    
    plt.tight_layout()
    return fig

def create_2d_projections(data, round_num=None):
    """Create 2D projections showing optimization trajectory."""
    fig, axes = plt.subplots(1, 3, figsize=(18, 5))
    
    if round_num is not None:
        # Plot specific round - show full Pareto front + best solution
        round_data = data['rounds'][round_num - 1]
        solutions = round_data['paretoFront']
        best = round_data['bestSolution']
        
        # Cost vs Disruption
        costs = [sol['cost'] for sol in solutions]
        disruptions = [sol['disruption'] for sol in solutions]
        axes[0].scatter(costs, disruptions, alpha=0.4, s=30, c='lightblue', label='Pareto Front')
        axes[0].scatter([best['cost']], [best['disruption']], 
                       c='red', s=200, marker='*', label='Best Solution', zorder=5)
        axes[0].set_xlabel('Cost (normalized)')
        axes[0].set_ylabel('Disruption (normalized)')
        axes[0].set_title('Cost vs Disruption')
        axes[0].legend()
        axes[0].grid(True, alpha=0.3)
        
        # Cost vs Balance
        balances = [sol['balance'] for sol in solutions]
        axes[1].scatter(costs, balances, alpha=0.4, s=30, c='lightblue', label='Pareto Front')
        axes[1].scatter([best['cost']], [best['balance']], 
                       c='red', s=200, marker='*', label='Best Solution', zorder=5)
        axes[1].set_xlabel('Cost (normalized)')
        axes[1].set_ylabel('Balance (normalized)')
        axes[1].set_title('Cost vs Balance')
        axes[1].legend()
        axes[1].grid(True, alpha=0.3)
        
        # Disruption vs Balance
        axes[2].scatter(disruptions, balances, alpha=0.4, s=30, c='lightblue', label='Pareto Front')
        axes[2].scatter([best['disruption']], [best['balance']], 
                       c='red', s=200, marker='*', label='Best Solution', zorder=5)
        axes[2].set_xlabel('Disruption (normalized)')
        axes[2].set_ylabel('Balance (normalized)')
        axes[2].set_title('Disruption vs Balance')
        axes[2].legend()
        axes[2].grid(True, alpha=0.3)
        
        fig.suptitle(f'{data["testCase"]["name"]} - Round {round_num}\n2D Projections', 
                    fontsize=16, y=1.02)
    else:
        # Show ONLY best solutions from each round connected by lines
        best_costs = [round_data['bestSolution']['cost'] for round_data in data['rounds']]
        best_disruptions = [round_data['bestSolution']['disruption'] for round_data in data['rounds']]
        best_balances = [round_data['bestSolution']['balance'] for round_data in data['rounds']]
        rounds = list(range(1, len(data['rounds']) + 1))
        
        # Cost vs Disruption trajectory
        axes[0].plot(best_costs, best_disruptions, 'o-', linewidth=2.5, markersize=8, 
                    color='darkblue', alpha=0.8, label='Optimization Trajectory')
        axes[0].scatter([best_costs[0]], [best_disruptions[0]], 
                       c='green', s=150, marker='s', label='Initial Best', zorder=5)
        axes[0].scatter([best_costs[-1]], [best_disruptions[-1]], 
                       c='red', s=150, marker='*', label='Final Best', zorder=5)
        
        # Add round numbers as annotations
        for i, (cost, disruption) in enumerate(zip(best_costs, best_disruptions)):
            if i % 2 == 0 or i == len(best_costs) - 1:  # Annotate every other round + final
                axes[0].annotate(f'R{i+1}', (cost, disruption), xytext=(5, 5), 
                               textcoords='offset points', fontsize=9, alpha=0.7)
        
        axes[0].set_xlabel('Cost (normalized)')
        axes[0].set_ylabel('Disruption (normalized)')
        axes[0].set_title('Cost vs Disruption - Optimization Trajectory')
        axes[0].legend()
        axes[0].grid(True, alpha=0.3)
        
        # Cost vs Balance trajectory
        axes[1].plot(best_costs, best_balances, 'o-', linewidth=2.5, markersize=8, 
                    color='darkgreen', alpha=0.8, label='Optimization Trajectory')
        axes[1].scatter([best_costs[0]], [best_balances[0]], 
                       c='green', s=150, marker='s', label='Initial Best', zorder=5)
        axes[1].scatter([best_costs[-1]], [best_balances[-1]], 
                       c='red', s=150, marker='*', label='Final Best', zorder=5)
        
        # Add round numbers as annotations
        for i, (cost, balance) in enumerate(zip(best_costs, best_balances)):
            if i % 2 == 0 or i == len(best_costs) - 1:
                axes[1].annotate(f'R{i+1}', (cost, balance), xytext=(5, 5), 
                               textcoords='offset points', fontsize=9, alpha=0.7)
        
        axes[1].set_xlabel('Cost (normalized)')
        axes[1].set_ylabel('Balance (normalized)')
        axes[1].set_title('Cost vs Balance - Optimization Trajectory')
        axes[1].legend()
        axes[1].grid(True, alpha=0.3)
        
        # Disruption vs Balance trajectory
        axes[2].plot(best_disruptions, best_balances, 'o-', linewidth=2.5, markersize=8, 
                    color='darkorange', alpha=0.8, label='Optimization Trajectory')
        axes[2].scatter([best_disruptions[0]], [best_balances[0]], 
                       c='green', s=150, marker='s', label='Initial Best', zorder=5)
        axes[2].scatter([best_disruptions[-1]], [best_balances[-1]], 
                       c='red', s=150, marker='*', label='Final Best', zorder=5)
        
        # Add round numbers as annotations
        for i, (disruption, balance) in enumerate(zip(best_disruptions, best_balances)):
            if i % 2 == 0 or i == len(best_disruptions) - 1:
                axes[2].annotate(f'R{i+1}', (disruption, balance), xytext=(5, 5), 
                               textcoords='offset points', fontsize=9, alpha=0.7)
        
        axes[2].set_xlabel('Disruption (normalized)')
        axes[2].set_ylabel('Balance (normalized)')
        axes[2].set_title('Disruption vs Balance - Optimization Trajectory')
        axes[2].legend()
        axes[2].grid(True, alpha=0.3)
        
        fig.suptitle(f'{data["testCase"]["name"]} - Best Solution Evolution\n'
                    f'Optimization Trajectory Across {len(data["rounds"])} Rounds', 
                    fontsize=16, y=1.02)
    
    plt.tight_layout()
    return fig

def create_3d_trajectory_plot(data):
    """Create 3D trajectory plot showing best solution evolution."""
    fig = plt.figure(figsize=(12, 9))
    ax = fig.add_subplot(111, projection='3d')
    
    # Extract best solutions from each round
    best_costs = [round_data['bestSolution']['cost'] for round_data in data['rounds']]
    best_disruptions = [round_data['bestSolution']['disruption'] for round_data in data['rounds']]
    best_balances = [round_data['bestSolution']['balance'] for round_data in data['rounds']]
    
    # Plot trajectory line
    ax.plot(best_costs, best_disruptions, best_balances, 
           'o-', linewidth=3, markersize=8, color='darkblue', alpha=0.8, label='Best Solution Trajectory')
    
    # Highlight start and end points
    ax.scatter([best_costs[0]], [best_disruptions[0]], [best_balances[0]], 
              c='green', s=200, marker='s', label='Initial Best', alpha=0.9)
    ax.scatter([best_costs[-1]], [best_disruptions[-1]], [best_balances[-1]], 
              c='red', s=200, marker='*', label='Final Best', alpha=0.9)
    
    # Add round number annotations for key points
    for i, (cost, disruption, balance) in enumerate(zip(best_costs, best_disruptions, best_balances)):
        if i == 0 or i == len(best_costs) - 1 or i % 3 == 0:  # Start, end, and every 3rd round
            ax.text(cost, disruption, balance, f'  R{i+1}', fontsize=9)
    
    ax.set_xlabel('Cost (normalized)', fontsize=12)
    ax.set_ylabel('Disruption (normalized)', fontsize=12)
    ax.set_zlabel('Balance (normalized)', fontsize=12)
    ax.set_title(f'{data["testCase"]["name"]} - 3D Optimization Trajectory\n'
                f'Best Solution Evolution Across {len(data["rounds"])} Rounds', 
                fontsize=14, pad=20)
    ax.legend()
    
    plt.tight_layout()
    return fig

def create_cluster_evolution_plot(data):
    """Create cluster state evolution plot showing real objective improvements."""
    fig, axes = plt.subplots(2, 3, figsize=(18, 10))
    
    rounds = list(range(1, len(data['rounds']) + 1))
    
    # Extract initial and final states for each round
    initial_costs = [round_data['initialState']['totalCost'] for round_data in data['rounds']]
    final_costs = [round_data['finalState']['totalCost'] for round_data in data['rounds']]
    initial_balances = [round_data['initialState']['balancePercent'] for round_data in data['rounds']]
    final_balances = [round_data['finalState']['balancePercent'] for round_data in data['rounds']]
    
    # Top row: Cluster state evolution
    axes[0, 0].plot(rounds, initial_costs, 'o--', linewidth=2, markersize=8, 
                   color='lightcoral', alpha=0.8, label='Before Optimization')
    axes[0, 0].plot(rounds, final_costs, 'o-', linewidth=3, markersize=8, 
                   color='darkgreen', alpha=0.9, label='After Optimization')
    axes[0, 0].set_title('Real Cluster Cost Evolution', fontsize=12, weight='bold')
    axes[0, 0].set_xlabel('Optimization Round')
    axes[0, 0].set_ylabel('Total Cost ($/hour)')
    axes[0, 0].legend()
    axes[0, 0].grid(True, alpha=0.3)
    
    axes[0, 1].plot(rounds, initial_balances, 'o--', linewidth=2, markersize=8, 
                   color='lightcoral', alpha=0.8, label='Before Optimization')
    axes[0, 1].plot(rounds, final_balances, 'o-', linewidth=3, markersize=8, 
                   color='darkgreen', alpha=0.9, label='After Optimization')
    axes[0, 1].set_title('Real Load Balance Evolution', fontsize=12, weight='bold')
    axes[0, 1].set_xlabel('Optimization Round')
    axes[0, 1].set_ylabel('Balance (% std dev)')
    axes[0, 1].legend()
    axes[0, 1].grid(True, alpha=0.3)
    
    # Cumulative improvements
    cost_savings = [round_data['improvements']['costSavings'] for round_data in data['rounds']]
    cumulative_savings = np.cumsum(cost_savings)
    balance_improvements = [round_data['improvements']['balanceImprovement'] for round_data in data['rounds']]
    cumulative_balance = np.cumsum(balance_improvements)
    
    axes[0, 2].bar(rounds, cost_savings, alpha=0.7, color='green', label='Per Round')
    axes[0, 2].plot(rounds, cumulative_savings, 'ro-', linewidth=2, markersize=6, 
                   label='Cumulative', zorder=5)
    axes[0, 2].set_title('Cost Savings Accumulation', fontsize=12, weight='bold')
    axes[0, 2].set_xlabel('Optimization Round')
    axes[0, 2].set_ylabel('Savings ($/hour)')
    axes[0, 2].legend()
    axes[0, 2].grid(True, alpha=0.3)
    
    # Bottom row: Objective space evolution (normalized values)
    best_costs_norm = [round_data['bestSolution']['cost'] for round_data in data['rounds']]
    best_disruptions_norm = [round_data['bestSolution']['disruption'] for round_data in data['rounds']]
    best_balances_norm = [round_data['bestSolution']['balance'] for round_data in data['rounds']]
    
    axes[1, 0].plot(rounds, best_costs_norm, 'o-', linewidth=3, markersize=8, 
                   color='darkblue', alpha=0.9, label='Cost Objective')
    axes[1, 0].set_title('Normalized Cost Objective Evolution', fontsize=12, weight='bold')
    axes[1, 0].set_xlabel('Optimization Round')
    axes[1, 0].set_ylabel('Cost (normalized, lower = better)')
    axes[1, 0].grid(True, alpha=0.3)
    
    axes[1, 1].plot(rounds, best_disruptions_norm, 'o-', linewidth=3, markersize=8, 
                   color='darkorange', alpha=0.9, label='Disruption Objective')
    axes[1, 1].set_title('Normalized Disruption Objective Evolution', fontsize=12, weight='bold')
    axes[1, 1].set_xlabel('Optimization Round')
    axes[1, 1].set_ylabel('Disruption (normalized, lower = better)')
    axes[1, 1].grid(True, alpha=0.3)
    
    axes[1, 2].plot(rounds, best_balances_norm, 'o-', linewidth=3, markersize=8, 
                   color='darkgreen', alpha=0.9, label='Balance Objective')
    axes[1, 2].set_title('Normalized Balance Objective Evolution', fontsize=12, weight='bold')
    axes[1, 2].set_xlabel('Optimization Round')
    axes[1, 2].set_ylabel('Balance (normalized, lower = better)')
    axes[1, 2].grid(True, alpha=0.3)
    
    # Add annotations for key improvements
    for i, (cost_save, round_num) in enumerate(zip(cost_savings, rounds)):
        if cost_save > max(cost_savings) * 0.5:  # Annotate significant savings
            axes[0, 2].annotate(f'${cost_save:.2f}', (round_num, cost_save), 
                              xytext=(0, 10), textcoords='offset points', 
                              ha='center', fontsize=9, weight='bold')
    
    plt.suptitle(f'{data["testCase"]["name"]} - Cluster Evolution Analysis\n'
                f'Real Cluster State Changes Across {len(data["rounds"])} Optimization Rounds', 
                fontsize=16, y=0.98)
    plt.tight_layout()
    return fig

def create_node_utilization_evolution(data):
    """Create node utilization evolution plot showing how workload distribution changes."""
    fig, axes = plt.subplots(2, 2, figsize=(16, 10))
    
    rounds = list(range(1, len(data['rounds']) + 1))
    node_names = [node['Name'] for node in data['testCase']['nodes']]
    
    # Extract utilization data for each round
    cpu_utils = []
    mem_utils = []
    pod_counts = []
    costs = []
    
    for round_data in data['rounds']:
        final_state = round_data['finalState']
        cpu_round = [node['cpuPercent'] for node in final_state['nodeUtilizations']]
        mem_round = [node['memPercent'] for node in final_state['nodeUtilizations']]
        pod_round = [node['podCount'] for node in final_state['nodeUtilizations']]
        cost_round = [node['hourlyCost'] for node in final_state['nodeUtilizations']]
        
        cpu_utils.append(cpu_round)
        mem_utils.append(mem_round)
        pod_counts.append(pod_round)
        costs.append(cost_round)
    
    # Create color map for nodes
    colors = plt.cm.Set3(np.linspace(0, 1, len(node_names)))
    
    # CPU Utilization Evolution
    for i, node_name in enumerate(node_names):
        node_cpu_evolution = [round_cpu[i] for round_cpu in cpu_utils]
        axes[0, 0].plot(rounds, node_cpu_evolution, 'o-', linewidth=2, markersize=6,
                       color=colors[i], label=node_name, alpha=0.8)
    axes[0, 0].set_title('CPU Utilization Evolution by Node', fontsize=12, weight='bold')
    axes[0, 0].set_xlabel('Optimization Round')
    axes[0, 0].set_ylabel('CPU Utilization (%)')
    axes[0, 0].legend(bbox_to_anchor=(1.05, 1), loc='upper left')
    axes[0, 0].grid(True, alpha=0.3)
    
    # Memory Utilization Evolution
    for i, node_name in enumerate(node_names):
        node_mem_evolution = [round_mem[i] for round_mem in mem_utils]
        axes[0, 1].plot(rounds, node_mem_evolution, 'o-', linewidth=2, markersize=6,
                       color=colors[i], label=node_name, alpha=0.8)
    axes[0, 1].set_title('Memory Utilization Evolution by Node', fontsize=12, weight='bold')
    axes[0, 1].set_xlabel('Optimization Round')
    axes[0, 1].set_ylabel('Memory Utilization (%)')
    axes[0, 1].legend(bbox_to_anchor=(1.05, 1), loc='upper left')
    axes[0, 1].grid(True, alpha=0.3)
    
    # Pod Count Evolution
    for i, node_name in enumerate(node_names):
        node_pod_evolution = [round_pods[i] for round_pods in pod_counts]
        axes[1, 0].plot(rounds, node_pod_evolution, 'o-', linewidth=2, markersize=6,
                       color=colors[i], label=node_name, alpha=0.8)
    axes[1, 0].set_title('Pod Count Evolution by Node', fontsize=12, weight='bold')
    axes[1, 0].set_xlabel('Optimization Round')
    axes[1, 0].set_ylabel('Number of Pods')
    axes[1, 0].legend(bbox_to_anchor=(1.05, 1), loc='upper left')
    axes[1, 0].grid(True, alpha=0.3)
    
    # Cost Distribution Evolution (which nodes are active)
    for i, node_name in enumerate(node_names):
        node_active = [1 if round_pods[i] > 0 else 0 for round_pods in pod_counts]
        node_cost_contrib = [node_active[j] * costs[j][i] for j in range(len(rounds))]
        axes[1, 1].plot(rounds, node_cost_contrib, 'o-', linewidth=2, markersize=6,
                       color=colors[i], label=node_name, alpha=0.8)
    axes[1, 1].set_title('Active Node Cost Contribution', fontsize=12, weight='bold')
    axes[1, 1].set_xlabel('Optimization Round')
    axes[1, 1].set_ylabel('Cost Contribution ($/hour)')
    axes[1, 1].legend(bbox_to_anchor=(1.05, 1), loc='upper left')
    axes[1, 1].grid(True, alpha=0.3)
    
    plt.suptitle(f'{data["testCase"]["name"]} - Node Utilization Evolution\n'
                f'How Workload Distribution Changes Across Optimization Rounds', 
                fontsize=16, y=0.98)
    plt.tight_layout()
    return fig

def create_intermediate_states_plot(data):
    """Create plot showing how intermediate states (after feasible moves) evolve."""
    fig, axes = plt.subplots(2, 3, figsize=(20, 10))
    
    rounds = list(range(1, len(data['rounds']) + 1))
    
    # Extract state progression: initial → intermediate → target
    initial_costs = [round_data['initialState']['totalCost'] for round_data in data['rounds']]
    intermediate_costs = [round_data['intermediateState']['totalCost'] for round_data in data['rounds']]
    target_costs = [round_data['finalState']['totalCost'] for round_data in data['rounds']]
    
    initial_balances = [round_data['initialState']['balancePercent'] for round_data in data['rounds']]
    intermediate_balances = [round_data['intermediateState']['balancePercent'] for round_data in data['rounds']]
    target_balances = [round_data['finalState']['balancePercent'] for round_data in data['rounds']]
    
    # Top row: Cost evolution with intermediate states
    axes[0, 0].plot(rounds, initial_costs, 'o--', linewidth=2, markersize=8, 
                   color='red', alpha=0.8, label='Current State')
    axes[0, 0].plot(rounds, intermediate_costs, 'o-', linewidth=3, markersize=8, 
                   color='orange', alpha=0.9, label='After Feasible Moves')
    axes[0, 0].plot(rounds, target_costs, 's:', linewidth=2, markersize=6, 
                   color='green', alpha=0.7, label='Target State (if no PDB)')
    axes[0, 0].set_title('Cost Evolution: Current → Feasible → Target', fontsize=12, weight='bold')
    axes[0, 0].set_xlabel('Optimization Round')
    axes[0, 0].set_ylabel('Total Cost ($/hour)')
    axes[0, 0].legend()
    axes[0, 0].grid(True, alpha=0.3)
    
    axes[0, 1].plot(rounds, initial_balances, 'o--', linewidth=2, markersize=8, 
                   color='red', alpha=0.8, label='Current State')
    axes[0, 1].plot(rounds, intermediate_balances, 'o-', linewidth=3, markersize=8, 
                   color='orange', alpha=0.9, label='After Feasible Moves')
    axes[0, 1].plot(rounds, target_balances, 's:', linewidth=2, markersize=6, 
                   color='green', alpha=0.7, label='Target State (if no PDB)')
    axes[0, 1].set_title('Balance Evolution: Current → Feasible → Target', fontsize=12, weight='bold')
    axes[0, 1].set_xlabel('Optimization Round')
    axes[0, 1].set_ylabel('Balance (% std dev)')
    axes[0, 1].legend()
    axes[0, 1].grid(True, alpha=0.3)
    
    # Feasible moves analysis
    total_moves = [round_data['feasibleMoves']['totalTargetMoves'] for round_data in data['rounds']]
    feasible_moves = [round_data['feasibleMoves']['feasibleMoves'] for round_data in data['rounds']]
    blocked_moves = [round_data['feasibleMoves']['blockedByPDB'] for round_data in data['rounds']]
    
    axes[0, 2].bar(rounds, feasible_moves, alpha=0.8, color='green', label='Feasible Moves')
    axes[0, 2].bar(rounds, blocked_moves, bottom=feasible_moves, alpha=0.8, color='red', label='Blocked by PDB')
    axes[0, 2].set_title('Move Feasibility Analysis', fontsize=12, weight='bold')
    axes[0, 2].set_xlabel('Optimization Round')
    axes[0, 2].set_ylabel('Number of Pod Moves')
    axes[0, 2].legend()
    axes[0, 2].grid(True, alpha=0.3)
    
    # Bottom row: Objective changes
    initial_obj_costs = [round_data['feasibleMoves']['objectiveChanges']['initialObjectives']['rawCost'] for round_data in data['rounds']]
    intermediate_obj_costs = [round_data['feasibleMoves']['objectiveChanges']['intermediateObjectives']['rawCost'] for round_data in data['rounds']]
    target_obj_costs = [round_data['feasibleMoves']['objectiveChanges']['targetObjectives']['rawCost'] for round_data in data['rounds']]
    
    axes[1, 0].plot(rounds, initial_obj_costs, 'o-', linewidth=2, markersize=6, color='red', label='Current')
    axes[1, 0].plot(rounds, intermediate_obj_costs, 'o-', linewidth=2, markersize=6, color='orange', label='Intermediate')
    axes[1, 0].plot(rounds, target_obj_costs, 's-', linewidth=2, markersize=6, color='green', label='Target')
    axes[1, 0].set_title('Objective Cost Evolution', fontsize=12, weight='bold')
    axes[1, 0].set_xlabel('Optimization Round')
    axes[1, 0].set_ylabel('Raw Cost ($/hour)')
    axes[1, 0].legend()
    axes[1, 0].grid(True, alpha=0.3)
    
    # Cumulative feasible moves
    cumulative_feasible = np.cumsum(feasible_moves)
    cumulative_blocked = np.cumsum(blocked_moves)
    
    axes[1, 1].plot(rounds, cumulative_feasible, 'o-', linewidth=3, markersize=8, 
                   color='green', alpha=0.9, label='Cumulative Feasible Moves')
    axes[1, 1].plot(rounds, cumulative_blocked, 'o-', linewidth=2, markersize=6, 
                   color='red', alpha=0.7, label='Cumulative Blocked Moves')
    axes[1, 1].set_title('Cumulative Move Implementation', fontsize=12, weight='bold')
    axes[1, 1].set_xlabel('Optimization Round')
    axes[1, 1].set_ylabel('Cumulative Pod Moves')
    axes[1, 1].legend()
    axes[1, 1].grid(True, alpha=0.3)
    
    # Feasibility percentage over time
    feasibility_percentages = [round_data['feasibleMoves']['feasibilityPercent'] for round_data in data['rounds']]
    axes[1, 2].plot(rounds, feasibility_percentages, 'o-', linewidth=3, markersize=8, 
                   color='purple', alpha=0.9)
    axes[1, 2].set_title('Move Feasibility Rate', fontsize=12, weight='bold')
    axes[1, 2].set_xlabel('Optimization Round')
    axes[1, 2].set_ylabel('Feasible Moves (%)')
    axes[1, 2].set_ylim(0, 100)
    axes[1, 2].grid(True, alpha=0.3)
    
    plt.suptitle(f'{data["testCase"]["name"]} - Intermediate States Analysis\n'
                f'Tracking Real Cluster Changes vs Algorithmic Targets', 
                fontsize=16, y=0.98)
    plt.tight_layout()
    return fig

def create_convergence_plot(data):
    """Create convergence analysis plot."""
    fig, axes = plt.subplots(2, 2, figsize=(15, 10))
    
    rounds = list(range(1, len(data['rounds']) + 1))
    
    # Best solution evolution
    best_costs = [round_data['bestSolution']['cost'] for round_data in data['rounds']]
    best_disruptions = [round_data['bestSolution']['disruption'] for round_data in data['rounds']]
    best_balances = [round_data['bestSolution']['balance'] for round_data in data['rounds']]
    best_weighted = [round_data['bestSolution']['weightedScore'] for round_data in data['rounds']]
    
    axes[0, 0].plot(rounds, best_costs, 'o-', linewidth=2, markersize=6)
    axes[0, 0].set_title('Best Cost Evolution')
    axes[0, 0].set_xlabel('Round')
    axes[0, 0].set_ylabel('Cost (normalized)')
    axes[0, 0].grid(True, alpha=0.3)
    
    axes[0, 1].plot(rounds, best_disruptions, 'o-', linewidth=2, markersize=6, color='orange')
    axes[0, 1].set_title('Best Disruption Evolution')
    axes[0, 1].set_xlabel('Round')
    axes[0, 1].set_ylabel('Disruption (normalized)')
    axes[0, 1].grid(True, alpha=0.3)
    
    axes[1, 0].plot(rounds, best_balances, 'o-', linewidth=2, markersize=6, color='green')
    axes[1, 0].set_title('Best Balance Evolution')
    axes[1, 0].set_xlabel('Round')
    axes[1, 0].set_ylabel('Balance (normalized)')
    axes[1, 0].grid(True, alpha=0.3)
    
    axes[1, 1].plot(rounds, best_weighted, 'o-', linewidth=2, markersize=6, color='red')
    axes[1, 1].set_title('Best Weighted Score Evolution')
    axes[1, 1].set_xlabel('Round')
    axes[1, 1].set_ylabel('Weighted Score')
    axes[1, 1].grid(True, alpha=0.3)
    
    plt.suptitle(f'{data["testCase"]["name"]} - Convergence Analysis', fontsize=16, y=0.98)
    plt.tight_layout()
    return fig

def create_cost_analysis_plot(data):
    """Create real cost and balance analysis."""
    fig, axes = plt.subplots(2, 2, figsize=(15, 10))
    
    rounds = list(range(1, len(data['rounds']) + 1))
    
    # Real cost evolution
    initial_costs = [round_data['initialState']['totalCost'] for round_data in data['rounds']]
    final_costs = [round_data['finalState']['totalCost'] for round_data in data['rounds']]
    cost_savings = [round_data['improvements']['costSavings'] for round_data in data['rounds']]
    
    axes[0, 0].plot(rounds, initial_costs, 'o-', label='Initial Cost', linewidth=2)
    axes[0, 0].plot(rounds, final_costs, 's-', label='Final Cost', linewidth=2)
    axes[0, 0].set_title('Real Cost Evolution ($/hour)')
    axes[0, 0].set_xlabel('Round')
    axes[0, 0].set_ylabel('Cost ($/hour)')
    axes[0, 0].legend()
    axes[0, 0].grid(True, alpha=0.3)
    
    axes[0, 1].bar(rounds, cost_savings, alpha=0.7)
    axes[0, 1].set_title('Cost Savings per Round')
    axes[0, 1].set_xlabel('Round')
    axes[0, 1].set_ylabel('Savings ($/hour)')
    axes[0, 1].grid(True, alpha=0.3)
    
    # Balance evolution
    initial_balance = [round_data['initialState']['balancePercent'] for round_data in data['rounds']]
    final_balance = [round_data['finalState']['balancePercent'] for round_data in data['rounds']]
    balance_improvement = [round_data['improvements']['balanceImprovement'] for round_data in data['rounds']]
    
    axes[1, 0].plot(rounds, initial_balance, 'o-', label='Initial Balance', linewidth=2)
    axes[1, 0].plot(rounds, final_balance, 's-', label='Final Balance', linewidth=2)
    axes[1, 0].set_title('Load Balance Evolution (%)')
    axes[1, 0].set_xlabel('Round')
    axes[1, 0].set_ylabel('Balance (%)')
    axes[1, 0].legend()
    axes[1, 0].grid(True, alpha=0.3)
    
    axes[1, 1].bar(rounds, balance_improvement, alpha=0.7, color='green')
    axes[1, 1].set_title('Balance Improvement per Round')
    axes[1, 1].set_xlabel('Round')
    axes[1, 1].set_ylabel('Improvement (percentage points)')
    axes[1, 1].grid(True, alpha=0.3)
    
    plt.suptitle(f'{data["testCase"]["name"]} - Real Cost & Balance Analysis', fontsize=16, y=0.98)
    plt.tight_layout()
    return fig

def create_final_solution_evolution_plots(data):
    """Create 2D plots showing final solution evolution across rounds for Cost vs Balance, Cost vs Disruption, Balance vs Disruption."""
    fig, axes = plt.subplots(1, 3, figsize=(18, 6))
    
    # Extract final solutions from each round
    rounds = []
    costs = []
    disruptions = []
    balances = []
    raw_costs = []
    
    for i, round_data in enumerate(data['rounds']):
        best_sol = round_data['bestSolution']
        rounds.append(i + 1)
        costs.append(best_sol['cost'])
        disruptions.append(best_sol['disruption'])
        balances.append(best_sol['balance'])
        raw_costs.append(best_sol.get('rawCost', 0))
    
    # Color by round progression
    colors = plt.cm.viridis(np.linspace(0, 1, len(rounds)))
    
    # Cost vs Balance
    scatter1 = axes[0].scatter(costs, balances, c=rounds, cmap='viridis', s=100, alpha=0.8, edgecolors='black', linewidth=0.5)
    axes[0].plot(costs, balances, '-', alpha=0.6, color='gray', linewidth=1)
    
    # Annotate start and end points
    axes[0].annotate('Start', (costs[0], balances[0]), xytext=(10, 10), textcoords='offset points',
                    bbox=dict(boxstyle='round,pad=0.3', facecolor='lightgreen', alpha=0.8),
                    arrowprops=dict(arrowstyle='->', connectionstyle='arc3,rad=0'))
    if len(costs) > 1:
        axes[0].annotate('End', (costs[-1], balances[-1]), xytext=(-10, -20), textcoords='offset points',
                        bbox=dict(boxstyle='round,pad=0.3', facecolor='lightcoral', alpha=0.8),
                        arrowprops=dict(arrowstyle='->', connectionstyle='arc3,rad=0'))
    
    axes[0].set_xlabel('Cost (normalized)')
    axes[0].set_ylabel('Balance (normalized)')
    axes[0].set_title('Final Solution Evolution:\nCost vs Balance')
    axes[0].grid(True, alpha=0.3)
    
    # Cost vs Disruption
    scatter2 = axes[1].scatter(costs, disruptions, c=rounds, cmap='viridis', s=100, alpha=0.8, edgecolors='black', linewidth=0.5)
    axes[1].plot(costs, disruptions, '-', alpha=0.6, color='gray', linewidth=1)
    
    # Annotate start and end points
    axes[1].annotate('Start', (costs[0], disruptions[0]), xytext=(10, 10), textcoords='offset points',
                    bbox=dict(boxstyle='round,pad=0.3', facecolor='lightgreen', alpha=0.8),
                    arrowprops=dict(arrowstyle='->', connectionstyle='arc3,rad=0'))
    if len(costs) > 1:
        axes[1].annotate('End', (costs[-1], disruptions[-1]), xytext=(-10, -20), textcoords='offset points',
                        bbox=dict(boxstyle='round,pad=0.3', facecolor='lightcoral', alpha=0.8),
                        arrowprops=dict(arrowstyle='->', connectionstyle='arc3,rad=0'))
    
    axes[1].set_xlabel('Cost (normalized)')
    axes[1].set_ylabel('Disruption (normalized)')
    axes[1].set_title('Final Solution Evolution:\nCost vs Disruption')
    axes[1].grid(True, alpha=0.3)
    
    # Balance vs Disruption
    scatter3 = axes[2].scatter(balances, disruptions, c=rounds, cmap='viridis', s=100, alpha=0.8, edgecolors='black', linewidth=0.5)
    axes[2].plot(balances, disruptions, '-', alpha=0.6, color='gray', linewidth=1)
    
    # Annotate start and end points
    axes[2].annotate('Start', (balances[0], disruptions[0]), xytext=(10, 10), textcoords='offset points',
                    bbox=dict(boxstyle='round,pad=0.3', facecolor='lightgreen', alpha=0.8),
                    arrowprops=dict(arrowstyle='->', connectionstyle='arc3,rad=0'))
    if len(balances) > 1:
        axes[2].annotate('End', (balances[-1], disruptions[-1]), xytext=(-10, -20), textcoords='offset points',
                        bbox=dict(boxstyle='round,pad=0.3', facecolor='lightcoral', alpha=0.8),
                        arrowprops=dict(arrowstyle='->', connectionstyle='arc3,rad=0'))
    
    axes[2].set_xlabel('Balance (normalized)')
    axes[2].set_ylabel('Disruption (normalized)')
    axes[2].set_title('Final Solution Evolution:\nBalance vs Disruption')
    axes[2].grid(True, alpha=0.3)
    
    # Add colorbar
    cbar = plt.colorbar(scatter1, ax=axes, orientation='horizontal', pad=0.1, shrink=0.8)
    cbar.set_label('Optimization Round', fontsize=12)
    
    plt.suptitle(f'Best Solution Evolution Across Rounds: {data["testCase"]["name"]}', 
                 fontsize=16, fontweight='bold')
    plt.tight_layout()
    return fig

def create_movement_analysis_plot(data):
    """Create plots showing pod rescheduling activity per round."""
    fig, axes = plt.subplots(2, 2, figsize=(15, 10))
    
    # Extract movement data from each round
    rounds = []
    total_movements = []
    feasible_movements = []
    blocked_movements = []
    feasibility_percent = []
    
    for i, round_data in enumerate(data['rounds']):
        rounds.append(i + 1)
        
        # Get movement data
        total_movements.append(round_data['bestSolution']['movements'])
        
        # Get feasible movement data if available
        if 'feasibleMoves' in round_data:
            feasible_movements.append(round_data['feasibleMoves']['feasibleMoves'])
            blocked_movements.append(round_data['feasibleMoves']['blockedByPDB'])
            feasibility_percent.append(round_data['feasibleMoves']['feasibilityPercent'])
        else:
            # Fallback if feasible moves data not available
            feasible_movements.append(total_movements[-1])
            blocked_movements.append(0)
            feasibility_percent.append(100.0)
    
    # Plot 1: Total vs Feasible Movements
    width = 0.35
    x = np.array(rounds)
    
    axes[0, 0].bar(x - width/2, total_movements, width, label='Target Movements', alpha=0.8, color='lightblue')
    axes[0, 0].bar(x + width/2, feasible_movements, width, label='Feasible Movements', alpha=0.8, color='green')
    
    axes[0, 0].set_title('Pod Movements per Round\n(Target vs Feasible)', fontsize=12, fontweight='bold')
    axes[0, 0].set_xlabel('Round')
    axes[0, 0].set_ylabel('Number of Pod Movements')
    axes[0, 0].legend()
    axes[0, 0].grid(True, alpha=0.3)
    
    # Add value labels on bars
    for i, (total, feasible) in enumerate(zip(total_movements, feasible_movements)):
        axes[0, 0].text(rounds[i] - width/2, total + 0.5, str(total), ha='center', va='bottom', fontsize=9)
        axes[0, 0].text(rounds[i] + width/2, feasible + 0.5, str(feasible), ha='center', va='bottom', fontsize=9)
    
    # Plot 2: Blocked Movements
    axes[0, 1].bar(rounds, blocked_movements, alpha=0.7, color='red')
    axes[0, 1].set_title('Movements Blocked by PDB\n(Per Round)', fontsize=12, fontweight='bold')
    axes[0, 1].set_xlabel('Round')
    axes[0, 1].set_ylabel('Blocked Movements')
    axes[0, 1].grid(True, alpha=0.3)
    
    # Add value labels
    for i, blocked in enumerate(blocked_movements):
        if blocked > 0:
            axes[0, 1].text(rounds[i], blocked + 0.2, str(blocked), ha='center', va='bottom', fontsize=9)
    
    # Plot 3: Feasibility Percentage
    axes[1, 0].plot(rounds, feasibility_percent, 'o-', linewidth=3, markersize=8, color='purple')
    axes[1, 0].fill_between(rounds, feasibility_percent, alpha=0.3, color='purple')
    axes[1, 0].set_title('Movement Feasibility\n(% of Target Moves Executed)', fontsize=12, fontweight='bold')
    axes[1, 0].set_xlabel('Round')
    axes[1, 0].set_ylabel('Feasibility (%)')
    axes[1, 0].set_ylim(0, 105)
    axes[1, 0].grid(True, alpha=0.3)
    
    # Add horizontal line at 100%
    axes[1, 0].axhline(y=100, color='green', linestyle='--', alpha=0.7, label='Perfect Feasibility')
    axes[1, 0].legend()
    
    # Plot 4: Movement Trend Analysis
    if len(rounds) > 3:
        # Calculate moving average for trend
        window_size = min(3, len(total_movements))
        moving_avg_total = np.convolve(total_movements, np.ones(window_size)/window_size, mode='valid')
        moving_avg_feasible = np.convolve(feasible_movements, np.ones(window_size)/window_size, mode='valid')
        moving_avg_rounds = rounds[window_size-1:]
        
        axes[1, 1].plot(rounds, total_movements, 'o-', alpha=0.5, label='Target Movements', color='lightblue')
        axes[1, 1].plot(rounds, feasible_movements, 's-', alpha=0.5, label='Feasible Movements', color='green')
        axes[1, 1].plot(moving_avg_rounds, moving_avg_total, '-', linewidth=3, label='Target Trend', color='blue')
        axes[1, 1].plot(moving_avg_rounds, moving_avg_feasible, '-', linewidth=3, label='Feasible Trend', color='darkgreen')
        
        axes[1, 1].set_title(f'Movement Trends\n({window_size}-Round Moving Average)', fontsize=12, fontweight='bold')
        axes[1, 1].set_xlabel('Round')
        axes[1, 1].set_ylabel('Number of Movements')
        axes[1, 1].legend()
        axes[1, 1].grid(True, alpha=0.3)
    else:
        # Simple bar chart for few rounds
        axes[1, 1].bar(rounds, total_movements, alpha=0.7, color='lightblue', label='Total Movements')
        axes[1, 1].set_title('Total Movements per Round', fontsize=12, fontweight='bold')
        axes[1, 1].set_xlabel('Round')
        axes[1, 1].set_ylabel('Number of Movements')
        axes[1, 1].grid(True, alpha=0.3)
    
    plt.suptitle(f'Pod Rescheduling Activity: {data["testCase"]["name"]}', 
                 fontsize=16, fontweight='bold')
    plt.tight_layout()
    return fig

def create_quality_metrics_plot(data):
    """Create plots for hypervolume and sparsity metrics over time."""
    metrics = calculate_metrics_over_time(data)
    
    fig, axes = plt.subplots(2, 2, figsize=(15, 10))
    
    rounds = metrics['rounds']
    
    # Hypervolume evolution
    axes[0, 0].plot(rounds, metrics['hypervolume'], 'o-', linewidth=2, markersize=6, color='blue')
    axes[0, 0].set_title('Hypervolume Evolution\n(Higher = Better Coverage)', fontsize=12, fontweight='bold')
    axes[0, 0].set_xlabel('Round')
    axes[0, 0].set_ylabel('Hypervolume')
    axes[0, 0].grid(True, alpha=0.3)
    axes[0, 0].fill_between(rounds, metrics['hypervolume'], alpha=0.3, color='blue')
    
    # Add trend line for hypervolume
    if len(rounds) > 2:
        z = np.polyfit(rounds, metrics['hypervolume'], 1)
        p = np.poly1d(z)
        axes[0, 0].plot(rounds, p(rounds), "--", alpha=0.8, color='darkblue', 
                       label=f'Trend: {"↗" if z[0] > 0 else "↘"} {abs(z[0]):.4f}/round')
        axes[0, 0].legend()
    
    # Sparsity (diversity) evolution
    finite_sparsity = [s for s in metrics['sparsity'] if s != float('inf') and s < 100]
    finite_rounds = [r for r, s in zip(rounds, metrics['sparsity']) if s != float('inf') and s < 100]
    
    if finite_sparsity:
        axes[0, 1].plot(finite_rounds, finite_sparsity, 's-', linewidth=2, markersize=6, color='green')
        axes[0, 1].fill_between(finite_rounds, finite_sparsity, alpha=0.3, color='green')
    
    axes[0, 1].set_title('Solution Diversity (Sparsity)\n(Higher = More Diverse)', fontsize=12, fontweight='bold')
    axes[0, 1].set_xlabel('Round')
    axes[0, 1].set_ylabel('Average Crowding Distance')
    axes[0, 1].grid(True, alpha=0.3)
    
    # Pareto front size evolution
    axes[1, 0].bar(rounds, metrics['pareto_size'], alpha=0.7, color='orange')
    axes[1, 0].set_title('Pareto Front Size\n(Number of Non-Dominated Solutions)', fontsize=12, fontweight='bold')
    axes[1, 0].set_xlabel('Round')
    axes[1, 0].set_ylabel('Number of Solutions')
    axes[1, 0].grid(True, alpha=0.3)
    
    # Quality vs Size scatter
    valid_indices = [i for i, s in enumerate(metrics['sparsity']) if s != float('inf') and s < 100]
    if valid_indices:
        scatter_hypervolume = [metrics['hypervolume'][i] for i in valid_indices]
        scatter_sparsity = [metrics['sparsity'][i] for i in valid_indices]
        scatter_rounds = [metrics['rounds'][i] for i in valid_indices]
        
        scatter = axes[1, 1].scatter(scatter_hypervolume, scatter_sparsity, 
                                   c=scatter_rounds, cmap='viridis', s=100, alpha=0.7)
        axes[1, 1].set_xlabel('Hypervolume (Coverage)')
        axes[1, 1].set_ylabel('Sparsity (Diversity)')
        axes[1, 1].set_title('Quality Trade-off\n(Coverage vs Diversity)', fontsize=12, fontweight='bold')
        axes[1, 1].grid(True, alpha=0.3)
        
        # Add colorbar for rounds
        cbar = plt.colorbar(scatter, ax=axes[1, 1])
        cbar.set_label('Round')
        
        # Annotate best points
        if scatter_hypervolume and scatter_sparsity:
            best_coverage_idx = np.argmax(scatter_hypervolume)
            best_diversity_idx = np.argmax(scatter_sparsity)
            
            axes[1, 1].annotate(f'Best Coverage\n(Round {scatter_rounds[best_coverage_idx]})', 
                              (scatter_hypervolume[best_coverage_idx], scatter_sparsity[best_coverage_idx]),
                              xytext=(10, 10), textcoords='offset points', 
                              bbox=dict(boxstyle='round,pad=0.3', facecolor='yellow', alpha=0.7),
                              arrowprops=dict(arrowstyle='->', connectionstyle='arc3,rad=0'))
            
            if best_diversity_idx != best_coverage_idx:
                axes[1, 1].annotate(f'Best Diversity\n(Round {scatter_rounds[best_diversity_idx]})', 
                                  (scatter_hypervolume[best_diversity_idx], scatter_sparsity[best_diversity_idx]),
                                  xytext=(-10, -20), textcoords='offset points',
                                  bbox=dict(boxstyle='round,pad=0.3', facecolor='lightgreen', alpha=0.7),
                                  arrowprops=dict(arrowstyle='->', connectionstyle='arc3,rad=0'))
    
    plt.suptitle(f'Multi-Objective Quality Metrics: {data["testCase"]["name"]}', 
                 fontsize=16, fontweight='bold')
    plt.tight_layout()
    return fig

def create_summary_info(data):
    """Create summary information plot."""
    fig, ax = plt.subplots(figsize=(12, 8))
    ax.axis('off')
    
    # Test case info
    test_info = f"""
TEST CASE: {data['testCase']['name']}
Expected Behavior: {data['testCase']['expectedBehavior']}

ALGORITHM CONFIGURATION:
• Population Size: {data['algorithm']['populationSize']}
• Max Generations: {data['algorithm']['maxGenerations']}
• Crossover Probability: {data['algorithm']['crossoverProbability']}
• Mutation Probability: {data['algorithm']['mutationProbability']}
• Tournament Size: {data['algorithm']['tournamentSize']}
• Parallel Execution: {data['algorithm']['parallelExecution']}

OBJECTIVE WEIGHTS:
• Cost: {data['testCase']['weightProfile']['Cost']:.2f}
• Disruption: {data['testCase']['weightProfile']['Disruption']:.2f}
• Balance: {data['testCase']['weightProfile']['Balance']:.2f}

CLUSTER CONFIGURATION:
• Nodes: {len(data['testCase']['nodes'])}
• Pods: {len(data['testCase']['pods'])}
• Optimization Rounds: {len(data['rounds'])}

FINAL RESULTS:
• Total Cost Savings: ${data['finalResults']['totalCostSavings']:.2f}/hour
• Total Balance Gain: {data['finalResults']['totalBalanceGain']:.1f} percentage points
• Final Pareto Size: {data['finalResults']['finalParetoSize']} solutions
• Annual Savings Estimate: ${data['finalResults']['totalCostSavings'] * 24 * 365:.0f}
"""
    
    ax.text(0.05, 0.95, test_info, transform=ax.transAxes, fontsize=11, 
            verticalalignment='top', fontfamily='monospace',
            bbox=dict(boxstyle="round,pad=0.5", facecolor="lightgray", alpha=0.8))
    
    plt.title(f'Optimization Summary - {data["testCase"]["name"]}', fontsize=16, pad=20)
    plt.tight_layout()
    return fig

def main():
    if len(sys.argv) < 2:
        print("Usage: python visualize_pareto_front.py <json_file>")
        print("Example: python visualize_pareto_front.py optimization_results_*.json")
        sys.exit(1)
    
    json_file = sys.argv[1]
    if not os.path.exists(json_file):
        print(f"Error: File {json_file} not found")
        sys.exit(1)
    
    print(f"📊 Loading optimization data from {json_file}...")
    data = load_optimization_data(json_file)
    
    # Create output directory
    output_dir = Path(f"plots_{data['testCase']['name'].replace(' ', '_')}")
    output_dir.mkdir(exist_ok=True)
    
    print(f"🎨 Creating visualizations...")
    
    # 1. Summary information
    fig = create_summary_info(data)
    fig.savefig(output_dir / "00_summary.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 2. 3D Pareto front evolution (all solutions)
    fig = create_3d_pareto_plot(data)
    fig.savefig(output_dir / "01_pareto_3d_evolution.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 3. 3D trajectory (best solutions only)
    fig = create_3d_trajectory_plot(data)
    fig.savefig(output_dir / "02_trajectory_3d.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 4. Cluster evolution analysis (real state changes)
    fig = create_cluster_evolution_plot(data)
    fig.savefig(output_dir / "03_cluster_evolution.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 5. Intermediate states analysis (feasible moves tracking)
    fig = create_intermediate_states_plot(data)
    fig.savefig(output_dir / "04_intermediate_states.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 6. 2D trajectory projections (best solutions only)
    fig = create_2d_projections(data)
    fig.savefig(output_dir / "05_trajectory_2d.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 7. Convergence analysis
    fig = create_convergence_plot(data)
    fig.savefig(output_dir / "06_convergence_analysis.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 8. Node utilization evolution
    fig = create_node_utilization_evolution(data)
    fig.savefig(output_dir / "07_node_utilization_evolution.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 9. Cost analysis
    fig = create_cost_analysis_plot(data)
    fig.savefig(output_dir / "08_cost_balance_analysis.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 10. Final solution evolution (2D projections)
    fig = create_final_solution_evolution_plots(data)
    fig.savefig(output_dir / "09_final_solution_evolution.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 11. Pod movement analysis
    fig = create_movement_analysis_plot(data)
    fig.savefig(output_dir / "10_movement_analysis.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 12. Quality metrics (hypervolume and sparsity)
    fig = create_quality_metrics_plot(data)
    fig.savefig(output_dir / "11_quality_metrics.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 13. Individual round plots (for detailed analysis)
    print(f"📈 Creating individual round plots...")
    for i in range(min(5, len(data['rounds']))):  # First 5 rounds
        round_num = i + 1
        
        # 3D plot for this round
        fig = create_3d_pareto_plot(data, round_num)
        fig.savefig(output_dir / f"round_{round_num:02d}_pareto_3d.png", dpi=300, bbox_inches='tight')
        plt.close(fig)
        
        # 2D projections for this round
        fig = create_2d_projections(data, round_num)
        fig.savefig(output_dir / f"round_{round_num:02d}_pareto_2d.png", dpi=300, bbox_inches='tight')
        plt.close(fig)
    
    print(f"✅ All visualizations saved to: {output_dir}/")
    print(f"📁 Files created:")
    for file in sorted(output_dir.glob("*.png")):
        print(f"   - {file.name}")
    
    print(f"\n🎯 Key Insights:")
    print(f"   • Test Case: {data['testCase']['name']}")
    print(f"   • Final Cost Savings: ${data['finalResults']['totalCostSavings']:.2f}/hour")
    print(f"   • Annual Savings: ${data['finalResults']['totalCostSavings'] * 24 * 365:.0f}")
    print(f"   • Balance Improvement: {data['finalResults']['totalBalanceGain']:.1f} percentage points")
    print(f"   • Pareto Solutions: {data['finalResults']['finalParetoSize']} in final round")

if __name__ == "__main__":
    main()
